package service

import (
	"context"
	"fmt"
	"strings"

	"lrss/internal/model"
)

// CreateFolder creates a folder. Empty parentID string is treated as root (nil).
func (lib *Library) CreateFolder(ctx context.Context, name string, parentID *string) (model.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Folder{}, fmt.Errorf("folder name is required")
	}
	if parentID != nil && strings.TrimSpace(*parentID) == "" {
		parentID = nil
	}
	return lib.Folders.Create(ctx, name, parentID)
}

// RenameFolder renames a folder. Name is trimmed; empty is rejected.
func (lib *Library) RenameFolder(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("folder name is required")
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("folder id is required")
	}
	return lib.Folders.Rename(ctx, id, name)
}

// DeleteFolder removes a folder (feeds become unfiled via FK).
func (lib *Library) DeleteFolder(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("folder id is required")
	}
	return lib.Folders.Delete(ctx, id)
}

// SetFolderNsfw marks or unmarks a folder as sensitive.
func (lib *Library) SetFolderNsfw(ctx context.Context, folderID string, nsfw bool) error {
	if strings.TrimSpace(folderID) == "" {
		return fmt.Errorf("folder id is required")
	}
	return lib.Folders.SetNsfw(ctx, folderID, nsfw)
}

// MoveFeed assigns a feed to a folder. Empty folderID string means unfiled.
func (lib *Library) MoveFeed(ctx context.Context, feedID string, folderID *string) error {
	if strings.TrimSpace(feedID) == "" {
		return fmt.Errorf("feed id is required")
	}
	if folderID != nil && strings.TrimSpace(*folderID) == "" {
		folderID = nil
	}
	return lib.Feeds.SetFolder(ctx, feedID, folderID)
}

// SetPaused pauses or unpauses a feed.
func (lib *Library) SetPaused(ctx context.Context, feedID string, paused bool) error {
	if strings.TrimSpace(feedID) == "" {
		return fmt.Errorf("feed id is required")
	}
	return lib.Feeds.SetPaused(ctx, feedID, paused)
}
