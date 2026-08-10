package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Options controls how the database is opened.
type Options struct {
	// Path is the SQLite file path. Empty uses DefaultPath().
	Path string
	// SkipMigrate skips applying embedded migrations (tests only).
	SkipMigrate bool
}

// DB wraps *sql.DB with the resolved path.
type DB struct {
	SQL  *sql.DB
	Path string
}

// Open opens (or creates) the SQLite database, applies PRAGMAs and migrations.
// sqlite-vector extension loading is intentionally deferred to a later step (S2).
func Open(ctx context.Context, opts Options) (*DB, error) {
	path := opts.Path
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}

	// modernc.org/sqlite DSN
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Desktop app: single writer, small pool is enough.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := applyPragmas(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	if !opts.SkipMigrate {
		if err := Migrate(ctx, sqlDB); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}

	// Optional sqlite-vector (S2). Failure is non-fatal — FTS still works.
	tryLoadVectorExtension(ctx, sqlDB)

	return &DB{SQL: sqlDB, Path: path}, nil
}

// Close closes the underlying connection pool.
func (d *DB) Close() error {
	if d == nil || d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA temp_store = MEMORY`,
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}
