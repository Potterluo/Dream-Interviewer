package server

import (
	"embed"
	"io/fs"
)

// webFS is the built frontend, produced by `make build-web`:
//
//	cd web && pnpm build          # static export into web/out
//	cp -r web/out internal/server/dist
//
// The dist/ directory is committed with a .gitkeep so the embed compiles
// before the first frontend build; the SPA handler falls back to a
// friendly placeholder until then. Run `make build-web` to fill it.
//
//go:embed all:dist
var webFS embed.FS

// DistFS exposes the embedded frontend build for alternative delivery
// targets — the desktop shell mounts it as the Wails asset server root.
func DistFS() (fs.FS, error) {
	return fs.Sub(webFS, "dist")
}
