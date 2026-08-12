//go:build !production

package main

import "embed"

// Empty FS for `go test` / non-production builds so compile does not require frontend/dist.
// Dev mode serves the Vite dev server; release builds use assets_prod.go.
var assets embed.FS
