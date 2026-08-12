//go:build production

package main

import "embed"

// Production desktop builds embed the Vite output (see wails3 task *build* -tags production).
//
//go:embed all:frontend/dist
var assets embed.FS
