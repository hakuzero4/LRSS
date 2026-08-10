package vector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"lrss/internal/db"
)

// Index manages sqlite-vector init/quantize for article_embeddings.
type Index struct {
	db *sql.DB
}

// NewIndex wraps a sql.DB.
func NewIndex(sqlDB *sql.DB) *Index {
	return &Index{db: sqlDB}
}

// Ensure prepares vector_init for the configured dimension when extension is loaded.
func (idx *Index) Ensure(ctx context.Context, dimensions int) error {
	return db.EnsureVectorSession(ctx, idx.db, dimensions)
}

// Quantize rebuilds quantized index if ready rows exist.
func (idx *Index) Quantize(ctx context.Context) (int64, error) {
	return db.QuantizeEmbeddings(ctx, idx.db)
}

// UpsertReady stores a ready embedding for an article.
func (idx *Index) UpsertReady(ctx context.Context, articleID, model, contentHash string, dimensions int, vec []float32) error {
	if len(vec) != dimensions {
		return fmt.Errorf("vector length %d != dimensions %d", len(vec), dimensions)
	}
	blob := Float32ToBlob(vec)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := idx.db.ExecContext(ctx, `
		INSERT INTO article_embeddings (
			article_id, model, dimensions, embedding, content_hash, status, error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'ready', NULL, ?, ?)
		ON CONFLICT(article_id) DO UPDATE SET
			model = excluded.model,
			dimensions = excluded.dimensions,
			embedding = excluded.embedding,
			content_hash = excluded.content_hash,
			status = 'ready',
			error = NULL,
			updated_at = excluded.updated_at
	`, articleID, model, dimensions, blob, contentHash, now, now)
	return err
}

// MarkPending marks an article for (re)embedding.
func (idx *Index) MarkPending(ctx context.Context, articleID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := idx.db.ExecContext(ctx, `
		INSERT INTO article_embeddings (
			article_id, model, dimensions, embedding, content_hash, status, created_at, updated_at
		) VALUES (?, '', 0, NULL, '', 'pending', ?, ?)
		ON CONFLICT(article_id) DO UPDATE SET
			status = 'pending',
			error = NULL,
			updated_at = excluded.updated_at
	`, articleID, now, now)
	return err
}

// InvalidateAll marks all embeddings pending (model/dimension change).
func (idx *Index) InvalidateAll(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := idx.db.ExecContext(ctx, `
		UPDATE article_embeddings
		SET status = 'pending', embedding = NULL, error = NULL, updated_at = ?`, now)
	return err
}

// ScanHit is a vector neighbor.
type ScanHit struct {
	ArticleID string
	Distance  float64
}

// ScanNearest runs quantize_scan when extension loaded; otherwise brute-force Go cosine.
func (idx *Index) ScanNearest(ctx context.Context, query []float32, k int) ([]ScanHit, error) {
	if k <= 0 {
		k = 20
	}
	if db.VectorInfo().Loaded {
		return idx.scanExtension(ctx, query, k)
	}
	return idx.scanBruteForce(ctx, query, k)
}

func (idx *Index) scanExtension(ctx context.Context, query []float32, k int) ([]ScanHit, error) {
	if err := idx.Ensure(ctx, len(query)); err != nil {
		return nil, err
	}
	// Prefer quantize_scan; fall back to full_scan if quantize missing.
	blob := Float32ToBlob(query)
	rows, err := idx.db.QueryContext(ctx, `
		SELECT e.article_id, v.distance
		FROM vector_quantize_scan('article_embeddings', 'embedding', ?, ?) AS v
		JOIN article_embeddings AS e ON e.rowid = v.rowid
		WHERE e.status = 'ready'`, blob, k)
	if err != nil {
		rows, err = idx.db.QueryContext(ctx, `
			SELECT e.article_id, v.distance
			FROM vector_full_scan('article_embeddings', 'embedding', ?, ?) AS v
			JOIN article_embeddings AS e ON e.rowid = v.rowid
			WHERE e.status = 'ready'`, blob, k)
		if err != nil {
			return nil, fmt.Errorf("vector scan: %w", err)
		}
	}
	defer rows.Close()
	var hits []ScanHit
	for rows.Next() {
		var h ScanHit
		if err := rows.Scan(&h.ArticleID, &h.Distance); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

func (idx *Index) scanBruteForce(ctx context.Context, query []float32, k int) ([]ScanHit, error) {
	rows, err := idx.db.QueryContext(ctx, `
		SELECT article_id, embedding, dimensions
		FROM article_embeddings
		WHERE status = 'ready' AND embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type cand struct {
		id  string
		dist float64
	}
	var all []cand
	for rows.Next() {
		var id string
		var blob []byte
		var dim int
		if err := rows.Scan(&id, &blob, &dim); err != nil {
			return nil, err
		}
		if dim != len(query) {
			continue
		}
		vec, err := BlobToFloat32(blob, dim)
		if err != nil {
			continue
		}
		all = append(all, cand{id: id, dist: CosineDistance(query, vec)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Sort by distance ascending (simple insertion for small n).
	for i := 1; i < len(all); i++ {
		j := i
		for j > 0 && all[j].dist < all[j-1].dist {
			all[j], all[j-1] = all[j-1], all[j]
			j--
		}
	}
	if k > len(all) {
		k = len(all)
	}
	hits := make([]ScanHit, 0, k)
	for i := 0; i < k; i++ {
		hits = append(hits, ScanHit{ArticleID: all[i].id, Distance: all[i].dist})
	}
	return hits, nil
}
