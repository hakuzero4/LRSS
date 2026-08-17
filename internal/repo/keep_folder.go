package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"lrss/internal/id"
	"lrss/internal/model"
)

const keepFolderNameMax = 80

// KeepFolderRepo persists 精选 folders (max depth 2 under the virtual root).
type KeepFolderRepo struct {
	DB *sql.DB
}

// NewKeepFolderRepo constructs a keep-folder repository.
func NewKeepFolderRepo(db *sql.DB) *KeepFolderRepo {
	return &KeepFolderRepo{DB: db}
}

const keepFolderCols = `id, name, parent_id, sort_order, hint, created_at, updated_at`

func scanKeepFolder(row scannable) (model.KeepFolder, error) {
	var f model.KeepFolder
	var parent sql.NullString
	if err := row.Scan(&f.ID, &f.Name, &parent, &f.SortOrder, &f.Hint, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return model.KeepFolder{}, err
	}
	f.ParentID = strPtr(parent)
	return f, nil
}

func normalizeKeepFolderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("keep folder name is required")
	}
	if utf8.RuneCountInString(name) > keepFolderNameMax {
		return "", fmt.Errorf("keep folder name too long")
	}
	return name, nil
}

// nameTaken reports whether name already exists under parent (COLLATE NOCASE).
// excludeID skips that row (rename to same name). Rows are fully consumed.
func (r *KeepFolderRepo) nameTaken(ctx context.Context, name, parentID, excludeID string) (bool, error) {
	var n int
	var err error
	switch {
	case parentID == "" && excludeID == "":
		err = r.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM keep_folders WHERE name = ? COLLATE NOCASE AND parent_id IS NULL`,
			name).Scan(&n)
	case parentID == "" && excludeID != "":
		err = r.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM keep_folders WHERE name = ? COLLATE NOCASE AND parent_id IS NULL AND id != ?`,
			name, excludeID).Scan(&n)
	case parentID != "" && excludeID == "":
		err = r.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM keep_folders WHERE name = ? COLLATE NOCASE AND parent_id = ?`,
			name, parentID).Scan(&n)
	default:
		err = r.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM keep_folders WHERE name = ? COLLATE NOCASE AND parent_id = ? AND id != ?`,
			name, parentID, excludeID).Scan(&n)
	}
	if err != nil {
		return false, fmt.Errorf("keep folder name check: %w", err)
	}
	return n > 0, nil
}

// List returns all keep folders ordered by sort_order, then name.
// UnreadCount is unread articles in that folder only (not descendants).
func (r *KeepFolderRepo) List(ctx context.Context) ([]model.KeepFolder, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT kf.id, kf.name, kf.parent_id, kf.sort_order, kf.hint, kf.created_at, kf.updated_at,
		       COALESCE(u.unread, 0)
		FROM keep_folders kf
		LEFT JOIN (
			SELECT k.folder_id AS folder_id, COUNT(*) AS unread
			FROM article_keeps k
			JOIN articles a ON a.id = k.article_id
			WHERE a.is_read = 0 AND k.folder_id IS NOT NULL
			GROUP BY k.folder_id
		) u ON u.folder_id = kf.id
		ORDER BY kf.sort_order ASC, kf.name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("list keep folders: %w", err)
	}
	defer rows.Close()

	var out []model.KeepFolder
	for rows.Next() {
		var f model.KeepFolder
		var parent sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &parent, &f.SortOrder, &f.Hint, &f.CreatedAt, &f.UpdatedAt, &f.UnreadCount); err != nil {
			return nil, err
		}
		f.ParentID = strPtr(parent)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.KeepFolder{}
	}
	return out, nil
}

// Get loads one keep folder by id. UnreadCount is left 0.
func (r *KeepFolderRepo) Get(ctx context.Context, folderID string) (model.KeepFolder, error) {
	folderID = strings.TrimSpace(folderID)
	row := r.DB.QueryRowContext(ctx, `
		SELECT `+keepFolderCols+` FROM keep_folders WHERE id = ?`, folderID)
	f, err := scanKeepFolder(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.KeepFolder{}, fmt.Errorf("keep folder not found: %s", folderID)
		}
		return model.KeepFolder{}, fmt.Errorf("get keep folder: %w", err)
	}
	return f, nil
}

// Create inserts a keep folder. Empty parentID is first level under the virtual 精选 root.
// Depth is at most 2 (parent must itself be a root folder).
func (r *KeepFolderRepo) Create(ctx context.Context, name, parentID string) (model.KeepFolder, error) {
	name, err := normalizeKeepFolderName(name)
	if err != nil {
		return model.KeepFolder{}, err
	}
	parentID = strings.TrimSpace(parentID)
	if parentID != "" {
		parent, err := r.Get(ctx, parentID)
		if err != nil {
			return model.KeepFolder{}, err
		}
		if parent.ParentID != nil && strings.TrimSpace(*parent.ParentID) != "" {
			return model.KeepFolder{}, fmt.Errorf("keep folder depth exceeds 2")
		}
	}
	taken, err := r.nameTaken(ctx, name, parentID, "")
	if err != nil {
		return model.KeepFolder{}, err
	}
	if taken {
		return model.KeepFolder{}, fmt.Errorf("keep folder name already exists")
	}

	now := nowUTC()
	f := model.KeepFolder{
		ID:        id.New(),
		Name:      name,
		SortOrder: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	var parentArg any
	if parentID != "" {
		pid := parentID
		f.ParentID = &pid
		parentArg = parentID
	}
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO keep_folders (id, name, parent_id, sort_order, hint, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, parentArg, f.SortOrder, f.Hint, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return model.KeepFolder{}, fmt.Errorf("create keep folder: %w", err)
	}
	return f, nil
}

// Rename updates a keep folder's display name (unique per parent, COLLATE NOCASE).
func (r *KeepFolderRepo) Rename(ctx context.Context, folderID, name string) error {
	folderID = strings.TrimSpace(folderID)
	name, err := normalizeKeepFolderName(name)
	if err != nil {
		return err
	}
	cur, err := r.Get(ctx, folderID)
	if err != nil {
		return err
	}
	parentID := ""
	if cur.ParentID != nil {
		parentID = strings.TrimSpace(*cur.ParentID)
	}
	taken, err := r.nameTaken(ctx, name, parentID, folderID)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("keep folder name already exists")
	}
	res, err := r.DB.ExecContext(ctx, `
		UPDATE keep_folders SET name = ?, updated_at = ? WHERE id = ?`,
		name, nowUTC(), folderID)
	if err != nil {
		return fmt.Errorf("rename keep folder: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("keep folder not found: %s", folderID)
	}
	return nil
}

// Delete removes a folder. Child folders CASCADE; article_keeps.folder_id SET NULL.
func (r *KeepFolderRepo) Delete(ctx context.Context, folderID string) error {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return fmt.Errorf("keep folder not found: %s", folderID)
	}
	// Collect this folder and its children, then clear article_keeps before DELETE
	// (MaxOpenConns=1; also covers ALTER-added FKs that may not fire ON DELETE SET NULL).
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id FROM keep_folders WHERE id = ? OR parent_id = ?`, folderID, folderID)
	if err != nil {
		return fmt.Errorf("delete keep folder: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("delete keep folder: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("delete keep folder: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("delete keep folder: %w", err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("keep folder not found: %s", folderID)
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	if _, err := r.DB.ExecContext(ctx,
		`UPDATE article_keeps SET folder_id = NULL WHERE folder_id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("delete keep folder: %w", err)
	}

	res, err := r.DB.ExecContext(ctx, `DELETE FROM keep_folders WHERE id = ?`, folderID)
	if err != nil {
		return fmt.Errorf("delete keep folder: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("keep folder not found: %s", folderID)
	}
	return nil
}
