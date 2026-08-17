package db

import (
	"path/filepath"

	"lrss/internal/appdata"
)

const dbFileName = "lrss.db"

// DefaultPath returns the user-scoped SQLite path, e.g.
// Windows: %LOCALAPPDATA%/LRSS/data/lrss.db
// Dev builds (`wails3 task dev`, go test) use LRSS-dev instead.
func DefaultPath() (string, error) {
	dir, err := appdata.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dbFileName), nil
}
