package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

const (
	appDirName  = "LRSS"
	dbFileName  = "lrss.db"
	dataSubPath = "data"
)

// DefaultPath returns the user-scoped SQLite path, e.g.
// Windows: %LOCALAPPDATA%/LRSS/data/lrss.db
func DefaultPath() (string, error) {
	// Prefer XDG data home (works cross-platform via adrg/xdg).
	dir := filepath.Join(xdg.DataHome, appDirName, dataSubPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return filepath.Join(dir, dbFileName), nil
}
