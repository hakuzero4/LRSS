package repo

import (
	"context"
	"database/sql"
	"fmt"

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
		SELECT id, name, parent_id, sort_order, created_at, updated_at
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
		if err := rows.Scan(&f.ID, &f.Name, &parent, &f.SortOrder, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		f.ParentID = strPtr(parent)
		out = append(out, f)
	}
	if out == nil {
		out = []model.Folder{}
	}
	return out, rows.Err()
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
		INSERT INTO folders (id, name, parent_id, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, nullStr(f.ParentID), f.SortOrder, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return model.Folder{}, fmt.Errorf("create folder: %w", err)
	}
	return f, nil
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
