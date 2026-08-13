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

const folderCols = `id, name, parent_id, sort_order, is_nsfw, display_mode, created_at, updated_at`

func scanFolder(row interface{ Scan(dest ...any) error }) (model.Folder, error) {
	var f model.Folder
	var parent sql.NullString
	var nsfw int
	var mode string
	if err := row.Scan(&f.ID, &f.Name, &parent, &f.SortOrder, &nsfw, &mode, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return model.Folder{}, err
	}
	f.ParentID = strPtr(parent)
	f.IsNsfw = nsfw != 0
	f.DisplayMode = model.NormalizeFolderDisplayMode(mode)
	return f, nil
}

// List returns all folders ordered by sort_order, name.
func (r *FolderRepo) List(ctx context.Context) ([]model.Folder, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT `+folderCols+`
		FROM folders
		ORDER BY sort_order ASC, name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()

	var out []model.Folder
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if out == nil {
		out = []model.Folder{}
	}
	return out, rows.Err()
}

// FindByNameAndParent finds a folder with the given display name under parent.
// parentID nil/empty means root. Name match is case-insensitive (COLLATE NOCASE).
// Returns sql.ErrNoRows when not found.
func (r *FolderRepo) FindByNameAndParent(ctx context.Context, name string, parentID *string) (model.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Folder{}, sql.ErrNoRows
	}
	var row *sql.Row
	if parentID == nil || strings.TrimSpace(*parentID) == "" {
		row = r.DB.QueryRowContext(ctx, `
			SELECT `+folderCols+`
			FROM folders
			WHERE name = ? COLLATE NOCASE AND parent_id IS NULL
			ORDER BY sort_order ASC, created_at ASC
			LIMIT 1`, name)
	} else {
		row = r.DB.QueryRowContext(ctx, `
			SELECT `+folderCols+`
			FROM folders
			WHERE name = ? COLLATE NOCASE AND parent_id = ?
			ORDER BY sort_order ASC, created_at ASC
			LIMIT 1`, name, strings.TrimSpace(*parentID))
	}
	f, err := scanFolder(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Folder{}, sql.ErrNoRows
		}
		return model.Folder{}, fmt.Errorf("find folder by name: %w", err)
	}
	return f, nil
}

// Get loads one folder by id.
func (r *FolderRepo) Get(ctx context.Context, folderID string) (model.Folder, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT `+folderCols+` FROM folders WHERE id = ?`, folderID)
	f, err := scanFolder(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Folder{}, fmt.Errorf("folder not found: %s", folderID)
		}
		return model.Folder{}, fmt.Errorf("get folder: %w", err)
	}
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
	f.DisplayMode = model.FolderDisplayList
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO folders (id, name, parent_id, sort_order, is_nsfw, display_mode, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, nullStr(f.ParentID), f.SortOrder, boolToInt(f.IsNsfw), f.DisplayMode, f.CreatedAt, f.UpdatedAt,
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

// SetDisplayMode sets the article-list layout for a folder (list|cards).
func (r *FolderRepo) SetDisplayMode(ctx context.Context, folderID, mode string) error {
	mode = model.NormalizeFolderDisplayMode(mode)
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE folders SET display_mode = ?, updated_at = ? WHERE id = ?`,
		mode, now, folderID)
	if err != nil {
		return fmt.Errorf("set folder display mode: %w", err)
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
