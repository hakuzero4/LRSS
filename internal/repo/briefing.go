package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"lrss/internal/id"
	"lrss/internal/model"
)

// BriefingRepo persists AI briefings (智能汇报).
type BriefingRepo struct {
	DB *sql.DB
}

// NewBriefingRepo constructs a briefing repository.
func NewBriefingRepo(db *sql.DB) *BriefingRepo {
	return &BriefingRepo{DB: db}
}

const briefingCols = `id, created_at, status, locale, model, overview, error,
	article_count, omitted_count, is_read, is_starred, payload_json`

func scanBriefing(row scannable) (model.Briefing, error) {
	var b model.Briefing
	var modelNS, errNS sql.NullString
	var payloadJSON string
	var isRead, isStarred int
	if err := row.Scan(
		&b.ID, &b.CreatedAt, &b.Status, &b.Locale, &modelNS, &b.Overview, &errNS,
		&b.ArticleCount, &b.OmittedCount, &isRead, &isStarred, &payloadJSON,
	); err != nil {
		return model.Briefing{}, err
	}
	b.Model = modelNS.String
	b.Error = errNS.String
	b.IsRead = isRead != 0
	b.IsStarred = isStarred != 0
	if strings.TrimSpace(payloadJSON) != "" {
		if err := json.Unmarshal([]byte(payloadJSON), &b.Payload); err != nil {
			return model.Briefing{}, fmt.Errorf("decode briefing payload: %w", err)
		}
	}
	return b, nil
}

func marshalBriefingPayload(p model.BriefingPayload) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Insert stores a briefing. Empty ID/CreatedAt are filled (UTC RFC3339).
func (r *BriefingRepo) Insert(ctx context.Context, b *model.Briefing) error {
	if b.ID == "" {
		b.ID = id.New()
	}
	if b.CreatedAt == "" {
		b.CreatedAt = nowUTC()
	}
	payloadJSON, err := marshalBriefingPayload(b.Payload)
	if err != nil {
		return fmt.Errorf("insert briefing: %w", err)
	}
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO ai_briefings (
			id, created_at, status, locale, model, overview, error,
			article_count, omitted_count, is_read, is_starred, payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.CreatedAt, b.Status, b.Locale, nullIfEmpty(b.Model), b.Overview, nullIfEmpty(b.Error),
		b.ArticleCount, b.OmittedCount, boolToInt(b.IsRead), boolToInt(b.IsStarred), payloadJSON,
	)
	if err != nil {
		return fmt.Errorf("insert briefing: %w", err)
	}
	return nil
}

// UpdateGenerated writes generation result fields (pending → ready|error).
func (r *BriefingRepo) UpdateGenerated(ctx context.Context, id, status, modelName, overview, errMsg string, articleCount, omittedCount int, payload model.BriefingPayload) error {
	payloadJSON, err := marshalBriefingPayload(payload)
	if err != nil {
		return fmt.Errorf("update briefing: %w", err)
	}
	res, err := r.DB.ExecContext(ctx, `
		UPDATE ai_briefings
		SET status = ?, model = ?, overview = ?, error = ?,
		    article_count = ?, omitted_count = ?, payload_json = ?
		WHERE id = ?`,
		status, nullIfEmpty(modelName), overview, nullIfEmpty(errMsg),
		articleCount, omittedCount, payloadJSON, id,
	)
	if err != nil {
		return fmt.Errorf("update briefing: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("briefing not found: %s", id)
	}
	return nil
}

// Get loads one briefing by id.
func (r *BriefingRepo) Get(ctx context.Context, id string) (model.Briefing, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT `+briefingCols+` FROM ai_briefings WHERE id = ?`, id)
	b, err := scanBriefing(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Briefing{}, fmt.Errorf("briefing not found: %s", id)
		}
		return model.Briefing{}, fmt.Errorf("get briefing: %w", err)
	}
	return b, nil
}

// List returns briefings newest first. limit<=0 defaults to 50.
func (r *BriefingRepo) List(ctx context.Context, limit int) ([]model.Briefing, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+briefingCols+`
		FROM ai_briefings
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list briefings: %w", err)
	}
	defer rows.Close()

	out := []model.Briefing{}
	for rows.Next() {
		b, err := scanBriefing(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list briefings: %w", err)
	}
	return out, nil
}

// SetRead updates is_read.
func (r *BriefingRepo) SetRead(ctx context.Context, id string, read bool) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE ai_briefings SET is_read = ? WHERE id = ?`, boolToInt(read), id)
	if err != nil {
		return fmt.Errorf("set briefing read: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("briefing not found: %s", id)
	}
	return nil
}

// SetStarred updates is_starred.
func (r *BriefingRepo) SetStarred(ctx context.Context, id string, starred bool) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE ai_briefings SET is_starred = ? WHERE id = ?`, boolToInt(starred), id)
	if err != nil {
		return fmt.Errorf("set briefing starred: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("briefing not found: %s", id)
	}
	return nil
}

// Delete removes one briefing by id.
func (r *BriefingRepo) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("briefing id required")
	}
	res, err := r.DB.ExecContext(ctx, `DELETE FROM ai_briefings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete briefing: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("briefing not found: %s", id)
	}
	return nil
}

// UnreadCount returns how many briefings have is_read = 0.
func (r *BriefingRepo) UnreadCount(ctx context.Context) (int, error) {
	var n int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_briefings WHERE is_read = 0`).Scan(&n); err != nil {
		return 0, fmt.Errorf("unread briefing count: %w", err)
	}
	return n, nil
}

// PruneOld deletes unstarred briefings beyond keepUnstarred newest (default 30).
// Starred rows are never deleted. MaxOpenConns=1: consume the keep-id SELECT before DELETE.
func (r *BriefingRepo) PruneOld(ctx context.Context, keepUnstarred int) (int, error) {
	if keepUnstarred <= 0 {
		keepUnstarred = 30
	}

	rows, err := r.DB.QueryContext(ctx, `
		SELECT id FROM ai_briefings
		WHERE is_starred = 0
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, keepUnstarred)
	if err != nil {
		return 0, fmt.Errorf("prune briefings select: %w", err)
	}
	var keep []string
	for rows.Next() {
		var keepID string
		if err := rows.Scan(&keepID); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("prune briefings scan: %w", err)
		}
		keep = append(keep, keepID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("prune briefings rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("prune briefings close: %w", err)
	}

	if len(keep) == 0 {
		res, err := r.DB.ExecContext(ctx, `DELETE FROM ai_briefings WHERE is_starred = 0`)
		if err != nil {
			return 0, fmt.Errorf("prune briefings: %w", err)
		}
		n, _ := res.RowsAffected()
		return int(n), nil
	}

	placeholders := make([]string, len(keep))
	args := make([]any, len(keep))
	for i, keepID := range keep {
		placeholders[i] = "?"
		args[i] = keepID
	}
	q := `DELETE FROM ai_briefings WHERE is_starred = 0 AND id NOT IN (` + strings.Join(placeholders, ",") + `)`
	res, err := r.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("prune briefings: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
