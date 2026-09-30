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
	// Log to a FILE before anything else: windowsgui builds have no console,
	// so slog's default stdout writer would discard everything, including the
	// reason a boot failed. See bootstrap.go.
	openDesktopLog()
	config.SetupLogging("info")

	cfg := config.Load()
	// Desktop data belongs next to the user's other app data, not the working
	// directory — but NEVER unconditionally: if the preferred location cannot
	// be written to, fall back instead of refusing to start, and honour
	// APP_DATA_DIR as an explicit override. probe with a real file, because a
	// directory can exist yet reject writes.
	dir, problems := pickWritableDataDir()
	if dir == "" {
		lastDataDirProblems = problems
		fatalDesktop("找不到可写的数据目录", nil)
	}
	cfg.DataDir = dir
	if len(problems) > 0 {
		// Not fatal, but worth recording: it explains why the data is not
		// where the user expects it.
		slog.Warn("preferred data dirs unusable, using a fallback",
			"using", dir, "rejected", problems)
	}

	// WebView2 refuses to start if its profile directory cannot be created, so
	// create it up front and fail with a readable message rather than a
	// controller error from deep inside the runtime.
	webviewDataPath := filepath.Join(cfg.DataDir, "webview2")
	if err := os.MkdirAll(webviewDataPath, 0o755); err != nil {
		lastDataDirProblems = problems
		fatalDesktop("无法创建 WebView2 数据目录 "+webviewDataPath, err)
	}

	application, err := app.Boot(cfg)
	if err != nil {
		fatalDesktop("打开数据库失败", err)
	}
	defer application.Close()

	// Ephemeral loopback listener; the window is pointed at it below.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatalDesktop("无法监听本地端口", err)
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
			// WebView2 defaults its profile to %APPDATA%\<binary name>, which
			// is the SECOND hard dependency on a writable %APPDATA% (the first
			// is the data dir above). Where %APPDATA% is unavailable — roaming
			// profiles, OneDrive redirection, EDR, an inherited sandbox — the
			// default path is rejected and Wails exits with a controller
			// error, which on a windowsgui build is invisible.
			//
			// Pointing it inside our own data dir removes that dependency and
			// keeps every bit of desktop state in one place, so a portable
			// copy of the exe carries its profile too.
			WebviewUserDataPath: webviewDataPath,
		},
	})
	if err != nil {
		fatalDesktop("桌面窗口退出异常", err)
	}
}
