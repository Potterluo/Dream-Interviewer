//go:build desktop

// desktop — the Wails delivery target: the SAME Go server and embedded
// UI as cmd/server, rendered in a native window through the OS webview.
//
// Architecture: the server listens on an EPHEMERAL loopback port and the
// window opens on http://127.0.0.1:<port>. The asset server serves only
// a one-line redirect page. Streaming rides a real TCP socket this way —
// WebView2 buffers responses served through its custom-scheme handler,
// which would break SSE (chat streaming, live events). Cookies,
// EventSource, and websockets behave exactly as in a browser tab.
//
// Build (see `make desktop`):
//
//	go build -tags desktop,production ./cmd/desktop
//
// Platform notes: Windows needs no CGO (WebView2 ships with Win10/11).
// Linux needs webkit2gtk-4.1 dev packages; macOS needs the Xcode command
// line tools — both build with cgo enabled. For development with live
// reload, install the Wails CLI (`go install
// github.com/wailsapp/wails/v2/cmd/wails@latest`) and use `wails dev`.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/Potterluo/dream-interviewer/internal/app"
	"github.com/Potterluo/dream-interviewer/internal/buildinfo"
	"github.com/Potterluo/dream-interviewer/internal/config"
)

func main() {
	config.SetupLogging("info")

	cfg := config.Load()
	// Desktop data belongs next to the user's other app data, not the
	// working directory.
	if dir, err := os.UserConfigDir(); err == nil {
		cfg.DataDir = filepath.Join(dir, "github.com/Potterluo/dream-interviewer")
	}

	application, err := app.Boot(cfg)
	if err != nil {
		slog.Error("boot", "err", err)
		os.Exit(1)
	}
	defer application.Close()

	// Ephemeral loopback listener; the window is pointed at it below.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := application.Serve(ctx, ln); err != nil {
			slog.Error("serve", "err", err)
			cancel()
		}
	}()

	// The asset server exists only to bounce the window onto the real
	// origin — nothing else is ever served from wails://.
	startURL := fmt.Sprintf("http://127.0.0.1:%d/", port)
	redirectAssets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(fmt.Sprintf(
			`<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=%s">`+
				`</head><body style="background:#09090b"></body></html>`, startURL))},
	}

	slog.Info("desktop running", "url", startURL, "version", buildinfo.Version, "data", cfg.DataDir)
	err = wails.Run(&options.App{
		Title:     "Dream Interviewer",
		Width:     1280,
		Height:    832,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: redirectAssets,
		},
		// The context is kept for future bindings (native dialogs,
		// tray, single-instance lock).
		OnStartup: func(_ context.Context) {},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		slog.Error("desktop exited", "err", err)
		os.Exit(1)
	}
}
