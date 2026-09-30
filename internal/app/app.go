// Package app assembles the runtime from its long-lived pieces — config,
// store, event hub, HTTP server — in ONE place so both delivery targets
// share identical wiring:
//
//	cmd/server   headless single binary (serve over TCP)
//	cmd/desktop  Wails desktop shell (window → localhost server)
package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/config"
	"github.com/Potterluo/dream-interviewer/internal/events"
	"github.com/Potterluo/dream-interviewer/internal/server"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// App holds the booted runtime. Close must be called on shutdown.
type App struct {
	Cfg    *config.Config
	Store  store.Store
	Hub    *events.Hub
	Server *server.Server
}

// Boot opens the store (with migrations) and wires the server. It does
// NOT listen — callers choose their delivery: Server.Run(ctx) for a TCP
// bind, or Serve(ctx, ln) for the desktop shell.
func Boot(cfg *config.Config) (*App, error) {
	st, err := store.New(&store.StorageConfig{
		Type:        cfg.DBType,
		DSN:         cfg.DBDSN,
		AutoMigrate: cfg.AutoMigrate,
	}, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	// ONE hub, shared. The template built two by mistake (`events.New()`
	// inline for the server), which silently sent every server-published
	// event into a hub nobody was subscribed to — the live-update feed
	// would appear connected and never deliver anything.
	hub := events.New()
	srv := server.New(cfg, st, hub)

	// Resolve the interviewer model now: a configuration saved by an admin
	// lives in the database, and it has to take effect at boot without a
	// restart. This also logs what was resolved (never the key).
	if err := srv.ReloadEngine(context.Background()); err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("resolve interviewer model configuration: %w", err)
	}
	return &App{
		Cfg:    cfg,
		Store:  st,
		Hub:    hub,
		Server: srv,
	}, nil
}

// Close releases the store. Call once at shutdown.
func (a *App) Close() {
	_ = a.Store.Close()
}

// Serve serves the built handler on an EXISTING listener and blocks
// until ctx is canceled or the listener fails. The desktop shell uses
// this with an ephemeral loopback listener: streaming responses (SSE)
// must ride a real TCP socket — WebView2's custom-scheme handler
// buffers responses, which would turn chat into "nothing until done".
func (a *App) Serve(ctx context.Context, ln net.Listener) error {
	handler, err := a.Server.BuildHandler()
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
