package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Vector capability state for the opened DB.
type VectorStatus struct {
	Loaded  bool   `json:"loaded"`
	Version string `json:"version,omitempty"`
	Backend string `json:"backend,omitempty"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
}

var (
	vectorMu     sync.Mutex
	vectorStatus VectorStatus
)

// VectorInfo returns the last known vector extension status.
func VectorInfo() VectorStatus {
	vectorMu.Lock()
	defer vectorMu.Unlock()
	return vectorStatus
}

func setVectorStatus(s VectorStatus) {
	vectorMu.Lock()
	defer vectorMu.Unlock()
	vectorStatus = s
}

// ResolveVectorLibPath finds the sqlite-vector shared library for this OS/arch.
func ResolveVectorLibPath() (string, error) {
	if p := os.Getenv("LRSS_VECTOR_LIB"); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("LRSS_VECTOR_LIB not found: %s", p)
	}

	name := vectorLibFileName()
	platform := vectorPlatformDir()

	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "lib", name),
			filepath.Join(dir, name),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 5; i++ {
			candidates = append(candidates,
				filepath.Join(dir, "third_party", "sqlite-vector", platform, name),
				filepath.Join(dir, "lib", name),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c, nil
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("sqlite-vector library %q not found for %s (set LRSS_VECTOR_LIB)", name, platform)
}

func vectorLibFileName() string {
	switch runtime.GOOS {
	case "windows":
		return "vector.dll"
	case "darwin":
		return "vector.dylib"
	default:
		return "vector.so"
	}
}

func vectorPlatformDir() string {
	switch {
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return "windows_amd64"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return "darwin_arm64"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return "darwin_amd64"
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return "linux_amd64"
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		return "linux_arm64"
	default:
		return runtime.GOOS + "_" + runtime.GOARCH
	}
}

// tryLoadVectorExtension records whether the native library file is present.
//
// modernc.org/sqlite cannot safely load third-party loadable extensions on the
// shared connection (load_extension can block the sole pool connection forever).
// S2 therefore keeps Vector.Loaded=false and uses in-process cosine search
// (internal/vector). The DLL is still resolved so packaging and future CGO
// (mattn/go-sqlite3) wiring can pick it up.
func tryLoadVectorExtension(_ context.Context, _ *sql.DB) {
	path, err := ResolveVectorLibPath()
	if err != nil {
		setVectorStatus(VectorStatus{Loaded: false, Error: err.Error()})
		return
	}
	setVectorStatus(VectorStatus{
		Loaded: false,
		Path:   path,
		Error:  "sqlite-vector present but not loaded under modernc; in-process cosine active (see docs/embedding.md)",
	})
}

// EnsureVectorSession re-inits vector column metadata when extension is loaded.
func EnsureVectorSession(ctx context.Context, sqlDB *sql.DB, dimensions int) error {
	if !VectorInfo().Loaded {
		return fmt.Errorf("sqlite-vector not loaded")
	}
	if dimensions <= 0 {
		return fmt.Errorf("invalid dimensions %d", dimensions)
	}
	opts := fmt.Sprintf("type=FLOAT32,dimension=%d,distance=COSINE", dimensions)
	_, err := sqlDB.ExecContext(ctx,
		`SELECT vector_init('article_embeddings', 'embedding', ?)`, opts)
	if err != nil {
		return fmt.Errorf("vector_init: %w", err)
	}
	return nil
}

// QuantizeEmbeddings runs TurboQuant when extension is loaded and ready rows exist.
func QuantizeEmbeddings(ctx context.Context, sqlDB *sql.DB) (int64, error) {
	if !VectorInfo().Loaded {
		return 0, fmt.Errorf("sqlite-vector not loaded")
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM article_embeddings WHERE status = 'ready' AND embedding IS NOT NULL`,
	).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	var quantized int64
	err := sqlDB.QueryRowContext(ctx,
		`SELECT vector_quantize('article_embeddings', 'embedding', 'qtype=TURBO,qbits=4')`,
	).Scan(&quantized)
	if err != nil {
		return 0, fmt.Errorf("vector_quantize: %w", err)
	}
	_, _ = sqlDB.ExecContext(ctx, `SELECT vector_quantize_preload('article_embeddings', 'embedding')`)
	return quantized, nil
}
