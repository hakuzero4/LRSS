package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"lrss/internal/id"
	"lrss/internal/model"
)

// FolderRepo persists folders.
type FolderRepo struct {
	DB *sql.DB
}

// NewFolderRepo constructs a folder repository.
func NewFolderRepo(db *sql.DB) *FolderRepo {
	return &FolderRepo{DB: db}
}

// List returns all folders ordered by sort_order, name.
func (r *FolderRepo) List(ctx context.Context) ([]model.Folder, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, name, parent_id, sort_order, is_nsfw, created_at, updated_at
		FROM folders
		ORDER BY sort_order ASC, name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()

	var out []model.Folder
	for rows.Next() {
		var f model.Folder
		var parent sql.NullString
		var nsfw int
		if err := rows.Scan(&f.ID, &f.Name, &parent, &f.SortOrder, &nsfw, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		f.ParentID = strPtr(parent)
		f.IsNsfw = nsfw != 0
		out = append(out, f)
	}
	if out == nil {
		out = []model.Folder{}
	}
	return out, rows.Err()
}

// Get loads one folder by id.
func (r *FolderRepo) Get(ctx context.Context, folderID string) (model.Folder, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT id, name, parent_id, sort_order, is_nsfw, created_at, updated_at
		FROM folders WHERE id = ?`, folderID)
	var f model.Folder
	var parent sql.NullString
	var nsfw int
	if err := row.Scan(&f.ID, &f.Name, &parent, &f.SortOrder, &nsfw, &f.CreatedAt, &f.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return model.Folder{}, fmt.Errorf("folder not found: %s", folderID)
		}
		return model.Folder{}, fmt.Errorf("get folder: %w", err)
	}
	f.ParentID = strPtr(parent)
	f.IsNsfw = nsfw != 0
	return f, nil
}

// Create inserts a folder and returns it with generated id/timestamps.
func (r *FolderRepo) Create(ctx context.Context, name string, parentID *string) (model.Folder, error) {
	now := nowUTC()
	f := model.Folder{
		ID:        id.New(),
		Name:      name,
		ParentID:  parentID,
		SortOrder: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO folders (id, name, parent_id, sort_order, is_nsfw, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, nullStr(f.ParentID), f.SortOrder, boolToInt(f.IsNsfw), f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return model.Folder{}, fmt.Errorf("create folder: %w", err)
	}
	return f, nil
}

// SetNsfw marks or unmarks a folder as sensitive (NSFW).
func (r *FolderRepo) SetNsfw(ctx context.Context, folderID string, nsfw bool) error {
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE folders SET is_nsfw = ?, updated_at = ? WHERE id = ?`,
		boolToInt(nsfw), now, folderID)
	if err != nil {
		return fmt.Errorf("set folder nsfw: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("folder not found: %s", folderID)
	}
	return nil
}

// Rename updates a folder's name. Name is trimmed; empty after trim is rejected.
func (r *FolderRepo) Rename(ctx context.Context, folderID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("folder name is required")
	}
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE folders SET name = ?, updated_at = ? WHERE id = ?`,
		name, now, folderID)
	if err != nil {
		return fmt.Errorf("rename folder: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("folder not found: %s", folderID)
	}
	return nil
}

// DeleteAll removes every folder. Prefer clearing feeds first (or rely on ON DELETE SET NULL).
func (r *FolderRepo) DeleteAll(ctx context.Context) (int, error) {
	var n int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM folders`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count folders: %w", err)
	}
	if _, err := r.DB.ExecContext(ctx, `DELETE FROM folders`); err != nil {
		return 0, fmt.Errorf("delete all folders: %w", err)
	}
	return n, nil
}

// Delete removes a folder (feeds.folder_id set null by FK).
func (r *FolderRepo) Delete(ctx context.Context, folderID string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM folders WHERE id = ?`, folderID)
	if err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("folder not found: %s", folderID)
	}
	return nil
}
