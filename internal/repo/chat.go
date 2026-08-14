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

// ChatRepo persists reading-assistant sessions.
type ChatRepo struct {
	DB *sql.DB
}

// NewChatRepo constructs a chat repository.
func NewChatRepo(db *sql.DB) *ChatRepo {
	return &ChatRepo{DB: db}
}

func marshalCitations(cites []model.ChatCitation) (string, error) {
	if len(cites) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(cites)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalCitations(raw string) []model.ChatCitation {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var out []model.ChatCitation
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func scanChatSession(row scannable) (model.ChatSession, error) {
	var s model.ChatSession
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt, &s.ArticleID, &s.CollectionID, &s.Locale); err != nil {
		return model.ChatSession{}, err
	}
	return s, nil
}

func scanChatMessage(row scannable) (model.ChatMessage, error) {
	var m model.ChatMessage
	var cites string
	if err := row.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &cites, &m.CreatedAt); err != nil {
		return model.ChatMessage{}, err
	}
	m.Citations = unmarshalCitations(cites)
	return m, nil
}

// GetOrCreateByArticle returns the unique session for articleID, creating one if needed.
func (r *ChatRepo) GetOrCreateByArticle(ctx context.Context, articleID, locale string) (model.ChatSession, error) {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return model.ChatSession{}, fmt.Errorf("article id is required")
	}
	if existing, err := r.GetByArticle(ctx, articleID); err == nil {
		if locale != "" && existing.Locale != locale {
			_ = r.TouchLocale(ctx, existing.ID, locale)
			existing.Locale = locale
		}
		return existing, nil
	} else if err != sql.ErrNoRows {
		return model.ChatSession{}, err
	}
	s := model.ChatSession{
		ID:        id.New(),
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
		ArticleID: articleID,
		Locale:    strings.TrimSpace(locale),
	}
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO ai_chat_sessions (id, created_at, updated_at, article_id, collection_id, locale)
		VALUES (?, ?, ?, ?, '', ?)`,
		s.ID, s.CreatedAt, s.UpdatedAt, s.ArticleID, s.Locale,
	)
	if err != nil {
		// Unique race: load the winner.
		if got, gerr := r.GetByArticle(ctx, articleID); gerr == nil {
			return got, nil
		}
		return model.ChatSession{}, fmt.Errorf("insert chat session: %w", err)
	}
	return s, nil
}

// GetByArticle loads the session for an article, or sql.ErrNoRows.
func (r *ChatRepo) GetByArticle(ctx context.Context, articleID string) (model.ChatSession, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, article_id, collection_id, locale
		FROM ai_chat_sessions WHERE article_id = ?`, strings.TrimSpace(articleID))
	s, err := scanChatSession(row)
	if err != nil {
		return model.ChatSession{}, err
	}
	return s, nil
}

// Get loads a session by id.
func (r *ChatRepo) Get(ctx context.Context, sessionID string) (model.ChatSession, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, article_id, collection_id, locale
		FROM ai_chat_sessions WHERE id = ?`, strings.TrimSpace(sessionID))
	return scanChatSession(row)
}

// TouchLocale updates locale + updated_at.
func (r *ChatRepo) TouchLocale(ctx context.Context, sessionID, locale string) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE ai_chat_sessions SET locale = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(locale), nowUTC(), sessionID)
	return err
}

// Touch bumps updated_at.
func (r *ChatRepo) Touch(ctx context.Context, sessionID string) error {
	_, err := r.DB.ExecContext(ctx, `
		UPDATE ai_chat_sessions SET updated_at = ? WHERE id = ?`, nowUTC(), sessionID)
	return err
}

// ListMessages returns messages oldest-first.
func (r *ChatRepo) ListMessages(ctx context.Context, sessionID string) ([]model.ChatMessage, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, session_id, role, content, citations_json, created_at
		FROM ai_chat_messages WHERE session_id = ? ORDER BY created_at ASC, id ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()
	var out []model.ChatMessage
	for rows.Next() {
		m, err := scanChatMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertMessage stores a turn. Empty ID/CreatedAt are filled.
func (r *ChatRepo) InsertMessage(ctx context.Context, m *model.ChatMessage) error {
	if m.ID == "" {
		m.ID = id.New()
	}
	if m.CreatedAt == "" {
		m.CreatedAt = nowUTC()
	}
	cites, err := marshalCitations(m.Citations)
	if err != nil {
		return fmt.Errorf("insert chat message: %w", err)
	}
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO ai_chat_messages (id, session_id, role, content, citations_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.ID, m.SessionID, m.Role, m.Content, cites, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert chat message: %w", err)
	}
	_ = r.Touch(ctx, m.SessionID)
	return nil
}

// DeleteByArticle removes the session (CASCADE messages) for an article.
func (r *ChatRepo) DeleteByArticle(ctx context.Context, articleID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM ai_chat_sessions WHERE article_id = ?`, strings.TrimSpace(articleID))
	if err != nil {
		return fmt.Errorf("delete chat session: %w", err)
	}
	return nil
}
