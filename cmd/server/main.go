// github.com/Potterluo/dream-interviewer — a full-stack web application starter: Go backend +
// Next.js frontend compiled into a single static binary.
//
// Command layout is deliberately tiny: the default action runs the
// server; -version prints build info. Add cobra subcommands only when
// you actually accumulate CLI surface.
//
// Desktop delivery lives in cmd/desktop (Wails) and shares the boot
// wiring via internal/app.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Potterluo/dream-interviewer/internal/app"
	"github.com/Potterluo/dream-interviewer/internal/buildinfo"
	"github.com/Potterluo/dream-interviewer/internal/config"
)

// Stamped by -ldflags at build time (see Makefile).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	port := flag.Int("port", 0, "override APP_PORT")
	dataDir := flag.String("data-dir", "", "override APP_DATA_DIR")
	devProxy := flag.String("dev-proxy", "", "override APP_DEV_PROXY (e.g. http://localhost:3000)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("github.com/Potterluo/dream-interviewer %s (commit %s, built %s)\n", version, commit, date)
		return
	}
	// Keep buildinfo (read by /api/status) in sync with main's flags.
	buildinfo.Version, buildinfo.Commit, buildinfo.Date = version, commit, date

	cfg := config.Load()
	if *port != 0 {
		cfg.Port = *port
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *devProxy != "" {
		cfg.DevProxy = *devProxy
	}
	config.SetupLogging(cfg.LogLevel)

	// Context: canceled on SIGINT/SIGTERM — the server shuts down
	// gracefully (5s drain) when it fires.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.Boot(cfg)
	if err != nil {
		slog.Error("boot", "err", err)
		os.Exit(1)
	}
	defer application.Close()

	if err := application.Server.Run(ctx); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}
