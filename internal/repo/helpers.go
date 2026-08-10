package repo

import (
	"database/sql"
	"time"
)

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func nullStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// scannable is implemented by *sql.Row and *sql.Rows.
type scannable interface {
	Scan(dest ...any) error
}
