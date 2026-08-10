package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

const defaultMaxRunes = 6000

// BuildInput concatenates title/summary/body for embedding.
func BuildInput(title, summary, contentText string) string {
	parts := make([]string, 0, 3)
	if t := strings.TrimSpace(title); t != "" {
		parts = append(parts, t)
	}
	if s := strings.TrimSpace(summary); s != "" {
		parts = append(parts, s)
	}
	if c := strings.TrimSpace(contentText); c != "" {
		parts = append(parts, c)
	}
	return TruncateRunes(strings.Join(parts, "\n"), defaultMaxRunes)
}

// TruncateRunes limits string length by runes.
func TruncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	b.Grow(max * 2)
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// ContentHash is a stable hash of embedding input text.
func ContentHash(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
