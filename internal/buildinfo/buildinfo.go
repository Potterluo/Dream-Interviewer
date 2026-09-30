// Package buildinfo holds the version identity stamped at link time.
//
// Stamp via -ldflags (Makefile and Dockerfile both do this):
//
//	-X github.com/Potterluo/dream-interviewer/internal/buildinfo.Version=v1.2.3
//
// Surfaced through GET /api/status and the dashboard so a running
// binary can tell you what it is.
package buildinfo

// Overridden by -ldflags at build time; defaults are what `go run` /
// `go build` without flags produce.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
