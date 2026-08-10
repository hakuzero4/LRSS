package search

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"lrss/internal/db"
	"lrss/internal/embed"
	"lrss/internal/settings"
	"lrss/internal/vector"
)

// Options for Search.
type Options struct {
	Mode  string
	Limit int
	// ExcludeNsfw drops hits from feeds with is_nsfw=1 (office mode).
	ExcludeNsfw bool
}

// Hit is a unified search result.
type Hit struct {
	ArticleID string  `json:"articleId"`
	Title     string  `json:"title"`
	Score     float64 `json:"score"`
	Source    string  `json:"source"` // fts|vector|hybrid
	Snippet   string  `json:"snippet,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
}

// Result is the search response.
type Result struct {
	Hits     []Hit    `json:"hits"`
	ModeUsed string   `json:"modeUsed"`
	Warnings []string `json:"warnings,omitempty"`
}

// Service performs FTS / vector / hybrid search.
type Service struct {
	SQL      *sql.DB
	Settings *settings.Store
	Index    *vector.Index
	// NewProvider builds embedder from current config (injected for tests).
	NewProvider func(settings.EmbeddingConfig) (embed.Provider, error)
}

// New creates a search service.
func New(sqlDB *sql.DB, store *settings.Store) *Service {
	return &Service{
		SQL:         sqlDB,
		Settings:    store,
		Index:       vector.NewIndex(sqlDB),
		NewProvider: embed.NewProvider,
	}
}

// Capabilities reports what search modes are available.
type Capabilities struct {
	FTS              bool   `json:"fts"`
	VectorExtension  bool   `json:"vectorExtension"`
	EmbeddingConfig  bool   `json:"embeddingConfigured"`
	VectorSearch     bool   `json:"vectorSearch"`
	Reason           string `json:"reason,omitempty"`
}

// Capabilities computes current search capabilities.
func (s *Service) Capabilities(ctx context.Context) (Capabilities, error) {
	cfg, err := s.Settings.LoadEmbeddingConfig(ctx)
	if err != nil {
		return Capabilities{}, err
	}
	ext := db.VectorInfo().Loaded
	configured := cfg.IsConfigured()
	// Vector search usable with extension OR pure-Go brute force when we have embeddings.
	vectorOK := configured // brute force works without extension
	reason := ""
	if !configured {
		reason = "向量模型未配置，使用全文检索"
	} else if !ext {
		reason = "sqlite-vector 扩展未加载，使用进程内余弦检索"
	}
	return Capabilities{
		FTS:             true,
		VectorExtension: ext,
		EmbeddingConfig: configured,
		VectorSearch:    vectorOK,
		Reason:          reason,
	}, nil
}

// Search runs the requested mode with auto-fallback.
func (s *Service) Search(ctx context.Context, query string, opts Options) (Result, error) {
	searchCfg, err := s.Settings.LoadSearchConfig(ctx)
	if err != nil {
		return Result{}, err
	}
	embCfg, err := s.Settings.LoadEmbeddingConfig(ctx)
	if err != nil {
		return Result{}, err
	}

	mode := opts.Mode
	if mode == "" {
		mode = searchCfg.Mode
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = searchCfg.FTSLimit
	}

	caps, _ := s.Capabilities(ctx)
	var warnings []string

	// Resolve auto
	if mode == settings.SearchModeAuto {
		if caps.VectorSearch {
			mode = settings.SearchModeHybrid
		} else {
			mode = settings.SearchModeFTS
		}
	}

	switch mode {
	case settings.SearchModeFTS:
		hits, err := s.searchFTS(ctx, query, limit)
		hits = s.filterNsfwHits(ctx, hits, opts.ExcludeNsfw)
		return Result{Hits: hits, ModeUsed: settings.SearchModeFTS, Warnings: warnings}, err

	case settings.SearchModeVector:
		if !caps.VectorSearch {
			warnings = append(warnings, "vector_unavailable")
			hits, err := s.searchFTS(ctx, query, limit)
			hits = s.filterNsfwHits(ctx, hits, opts.ExcludeNsfw)
			return Result{Hits: hits, ModeUsed: settings.SearchModeFTS, Warnings: warnings}, err
		}
		hits, err := s.searchVector(ctx, query, embCfg, searchCfg.VectorTopK)
		if err != nil {
			warnings = append(warnings, err.Error())
			fhits, ferr := s.searchFTS(ctx, query, limit)
			fhits = s.filterNsfwHits(ctx, fhits, opts.ExcludeNsfw)
			return Result{Hits: fhits, ModeUsed: settings.SearchModeFTS, Warnings: warnings}, ferr
		}
		if len(hits) > limit {
			hits = hits[:limit]
		}
		hits = s.filterNsfwHits(ctx, hits, opts.ExcludeNsfw)
		return Result{Hits: hits, ModeUsed: settings.SearchModeVector, Warnings: warnings}, nil

	case settings.SearchModeHybrid:
		if !caps.VectorSearch {
			warnings = append(warnings, "vector_unavailable")
			hits, err := s.searchFTS(ctx, query, limit)
			hits = s.filterNsfwHits(ctx, hits, opts.ExcludeNsfw)
			return Result{Hits: hits, ModeUsed: settings.SearchModeFTS, Warnings: warnings}, err
		}
		hits, err := s.searchHybrid(ctx, query, embCfg, searchCfg, limit)
		if err != nil {
			warnings = append(warnings, err.Error())
			fhits, ferr := s.searchFTS(ctx, query, limit)
			fhits = s.filterNsfwHits(ctx, fhits, opts.ExcludeNsfw)
			return Result{Hits: fhits, ModeUsed: settings.SearchModeFTS, Warnings: warnings}, ferr
		}
		hits = s.filterNsfwHits(ctx, hits, opts.ExcludeNsfw)
		return Result{Hits: hits, ModeUsed: settings.SearchModeHybrid, Warnings: warnings}, nil

	default:
		return Result{}, fmt.Errorf("unknown search mode %q", mode)
	}
}

// filterNsfwHits removes hits whose article belongs to an is_nsfw feed
// or a feed inside an is_nsfw folder (office mode).
func (s *Service) filterNsfwHits(ctx context.Context, hits []Hit, exclude bool) []Hit {
	if !exclude || len(hits) == 0 || s.SQL == nil {
		return hits
	}
	out := make([]Hit, 0, len(hits))
	for _, h := range hits {
		var n int
		err := s.SQL.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM articles a
			JOIN feeds f ON f.id = a.feed_id
			LEFT JOIN folders fo ON fo.id = f.folder_id
			WHERE a.id = ? AND (f.is_nsfw = 1 OR IFNULL(fo.is_nsfw, 0) = 1)`, h.ArticleID).Scan(&n)
		if err != nil || n > 0 {
			continue
		}
		out = append(out, h)
	}
	return out
}

func (s *Service) searchFTS(ctx context.Context, query string, limit int) ([]Hit, error) {
	raw, err := SearchFTS(ctx, s.SQL, query, limit)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(raw))
	for i, r := range raw {
		// lower rank number / bm25 is better — convert to score
		score := 1.0 / (1.0 + float64(i) + r.Rank)
		hits = append(hits, Hit{
			ArticleID: r.ArticleID,
			Title:     r.Title,
			Score:     score,
			Source:    "fts",
			Snippet:   r.Snippet,
		})
	}
	return hits, nil
}

func (s *Service) searchVector(ctx context.Context, query string, embCfg settings.EmbeddingConfig, topK int) ([]Hit, error) {
	p, err := s.NewProvider(embCfg)
	if err != nil {
		return nil, err
	}
	vecs, err := p.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	if err := s.Index.Ensure(ctx, embCfg.Dimensions); err != nil && db.VectorInfo().Loaded {
		// brute force path ignores ensure failure
		_ = err
	}
	raw, err := s.Index.ScanNearest(ctx, vecs[0], topK)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(raw))
	for i, r := range raw {
		title := s.lookupTitle(ctx, r.ArticleID)
		score := 1.0 / (1.0 + r.Distance + float64(i)*0.001)
		hits = append(hits, Hit{
			ArticleID: r.ArticleID,
			Title:     title,
			Score:     score,
			Source:    "vector",
			Distance:  r.Distance,
		})
	}
	return hits, nil
}

func (s *Service) searchHybrid(ctx context.Context, query string, embCfg settings.EmbeddingConfig, sc settings.SearchConfig, limit int) ([]Hit, error) {
	ftsHits, err := s.searchFTS(ctx, query, sc.FTSLimit)
	if err != nil {
		return nil, err
	}
	vecHits, err := s.searchVector(ctx, query, embCfg, sc.VectorTopK)
	if err != nil {
		// degrade to fts only
		return ftsHits, nil
	}
	return rrfMerge(ftsHits, vecHits, limit), nil
}

func (s *Service) lookupTitle(ctx context.Context, id string) string {
	var t string
	_ = s.SQL.QueryRowContext(ctx, `SELECT title FROM articles WHERE id = ?`, id).Scan(&t)
	return t
}

// rrfMerge Reciprocal Rank Fusion.
func rrfMerge(a, b []Hit, limit int) []Hit {
	const k = 60.0
	type acc struct {
		hit   Hit
		score float64
	}
	m := map[string]*acc{}
	add := func(list []Hit, source string) {
		for i, h := range list {
			e, ok := m[h.ArticleID]
			if !ok {
				cp := h
				cp.Source = "hybrid"
				e = &acc{hit: cp}
				m[h.ArticleID] = e
			}
			e.score += 1.0 / (k + float64(i+1))
			if e.hit.Title == "" {
				e.hit.Title = h.Title
			}
			if e.hit.Snippet == "" {
				e.hit.Snippet = h.Snippet
			}
			_ = source
		}
	}
	add(a, "fts")
	add(b, "vector")
	out := make([]Hit, 0, len(m))
	for _, e := range m {
		h := e.hit
		h.Score = e.score
		h.Source = "hybrid"
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
