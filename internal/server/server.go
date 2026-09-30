// Package server hosts the HTTP surface: the JSON API under /api/*, the
// WebSocket demo at /ws, health probes, and the embedded web UI.
//
// Layout conventions:
//
//   - server.go    — struct, route table, lifecycle (this file)
//   - middleware.go— logging / recovery / security headers / rate limit
//   - respond.go   — JSON write/read helpers (the ONE response shape)
//   - handlers_*.go— one file per domain, methods on *Server
//   - events.go/ws.go — realtime (SSE + WebSocket)
//   - spa.go       — embedded single-page app
//   - devproxy.go  — dev-time reverse proxy to `next dev`
//
// All handlers are http.HandlerFunc methods on *Server with the identity
// already resolved on ctx by the auth middleware (see protected/adminOnly).
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/buildinfo"
	"github.com/Potterluo/dream-interviewer/internal/config"
	"github.com/Potterluo/dream-interviewer/internal/events"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// Server owns every long-lived dependency. Built once in main via New.
type Server struct {
	cfg       *config.Config
	store     store.Store
	auth      *auth.Resolver
	hub       *events.Hub
	limiter   *rateLimiter
	startedAt time.Time
	// engine is swappable: an admin can change the model configuration at
	// runtime (settings.go), and a request already streaming keeps the
	// engine it started with.
	engine *engineHolder
	// advanceLocks serialises interview advances per interview id, so a
	// duplicated submit cannot buy two model calls (see keyedmutex.go).
	advanceLocks *keyedMutex
}

// New wires the server together.
//
// The engine is built from Config here so both delivery targets
// (cmd/server and cmd/desktop, via internal/app) start identically; any
// database-stored override is applied by ReloadEngine during boot.
func New(cfg *config.Config, st store.Store, hub *events.Hub) *Server {
	return &Server{
		cfg:     cfg,
		store:   st,
		auth:    auth.NewResolver(st),
		hub:     hub,
		limiter: newRateLimiter(cfg.RateLimitRPM),
		engine: newEngineHolder(buildEngine(llmSettings{
			Provider:    cfg.LLMProvider,
			BaseURL:     cfg.LLMBaseURL,
			Model:       cfg.LLMModel,
			APIKey:      cfg.LLMAPIKey,
			TimeoutSec:  cfg.LLMTimeoutSec,
			MaxTokens:   cfg.LLMMaxTokens,
			Temperature: cfg.LLMTemperature,
		})),
		startedAt:    time.Now(),
		advanceLocks: newKeyedMutex(),
	}
}

// ReloadEngine re-resolves the model configuration (database over env over
// default) and installs it. internal/app calls it once at boot so a
// saved configuration takes effect without a restart.
func (s *Server) ReloadEngine(ctx context.Context) error {
	_, _, err := s.reloadEngine(ctx)
	return err
}

// BuildHandler assembles the complete HTTP handler — API, WebSocket,
// health probes, SPA root. Run() serves it over TCP; the desktop target
// (cmd/desktop) hands it to the Wails asset server instead, so both
// delivery shapes share one route table.
func (s *Server) BuildHandler() (http.Handler, error) {
	mux := http.NewServeMux()

	// Health probes — unauthenticated, no middleware noise.
	healthz := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /livez", healthz)
	mux.HandleFunc("GET /readyz", healthz)

	// The /api/* router. Middleware runs once around the whole subtree;
	// per-route wrappers choose the auth level:
	//
	//   s.public(h)    — rate-limited, no auth required (bootstrap routes)
	//   s.protected(h) — auth required, rate-limited per user
	//   s.adminOnly(h) — protected + admin role
	//   s.writable(h)  — protected + rejects read-only actAs callers
	api := http.NewServeMux()

	// Bootstrap / session.
	api.HandleFunc("GET /api/status", s.public(s.handleStatus))
	api.HandleFunc("POST /api/onboard", s.public(s.handleOnboard))
	api.HandleFunc("POST /api/login", s.public(s.handleLogin))
	api.HandleFunc("POST /api/logout", s.protected(s.handleLogout))
	api.HandleFunc("GET /api/me", s.protected(s.handleMe))
	api.HandleFunc("PUT /api/me", s.writable(s.handleUpdateMe))
	// writable, not protected: changing your own password is a mutation, so
	// a read-only ?actAs= caller must be rejected like any other mutation.
	// Registered as protected, it broke the documented invariant that actAs
	// is read-only (the blast radius was only the caller's own account,
	// which is why it was easy to miss).
	api.HandleFunc("POST /api/me/password", s.writable(s.handleChangeMyPassword))

	// File uploads: bytes on disk, metadata in the DB. Download allows
	// ?token= for plain HTTP clients (see auth.queryTokenAllowed).
	api.HandleFunc("GET /api/files", s.protected(s.handleListFiles))
	api.HandleFunc("POST /api/files", s.writable(s.handleUploadFile))
	api.HandleFunc("GET /api/files/{id}", s.protected(s.handleDownloadFile))
	api.HandleFunc("DELETE /api/files/{id}", s.writable(s.handleDeleteFile))

	// Interviews — the product. CRUD plus ONE streaming endpoint that
	// drives the state machine (see handlers_interviews.go and
	// docs/API.md §3). The singular /api/interview/* namespace holds the
	// non-resource-scoped reads, so no literal path can ever shadow an
	// interview id.
	api.HandleFunc("GET /api/interviews", s.protected(s.handleListInterviews))
	api.HandleFunc("POST /api/interviews", s.writable(s.handleCreateInterview))
	api.HandleFunc("GET /api/interviews/{id}", s.protected(s.handleGetInterview))
	api.HandleFunc("PUT /api/interviews/{id}", s.writable(s.handleUpdateInterview))
	api.HandleFunc("DELETE /api/interviews/{id}", s.writable(s.handleDeleteInterview))
	api.HandleFunc("POST /api/interviews/{id}/advance", s.writable(s.handleAdvanceInterview))
	api.HandleFunc("GET /api/interviews/{id}/export", s.protected(s.handleExportInterview))

	api.HandleFunc("GET /api/interview/stats", s.protected(s.handleInterviewStats))
	api.HandleFunc("GET /api/interview/presets", s.protected(s.handleInterviewPresets))
	api.HandleFunc("GET /api/interview/engine", s.protected(s.handleInterviewEngine))

	// Admin: runtime configuration and shared presets. Everything under
	// /api/admin/ is adminOnly AND re-checks the role in-handler.
	api.HandleFunc("GET /api/admin/settings", s.adminOnly(s.handleGetAdminSettings))
	api.HandleFunc("PUT /api/admin/settings", s.adminOnly(s.writable(s.handleUpdateAdminSettings)))
	api.HandleFunc("DELETE /api/admin/settings", s.adminOnly(s.writable(s.handleDeleteAdminSettings)))
	api.HandleFunc("POST /api/admin/settings/test", s.adminOnly(s.writable(s.handleTestAdminSettings)))

	api.HandleFunc("POST /api/admin/presets", s.adminOnly(s.writable(s.handleCreatePreset)))
	api.HandleFunc("PUT /api/admin/presets/{id}", s.adminOnly(s.writable(s.handleUpdatePreset)))
	api.HandleFunc("DELETE /api/admin/presets/{id}", s.adminOnly(s.writable(s.handleDeletePreset)))

	api.HandleFunc("GET /api/admin/users/{id}/overview", s.adminOnly(s.handleUserOverview))

	// Realtime.
	api.HandleFunc("GET /api/events", s.protected(s.handleEvents))

	// Generated entity routes (see `generator entity` in cmd/generator).
	// --- gen:routes ---

	// Admin: users.
	api.HandleFunc("GET /api/users", s.adminOnly(s.handleListUsers))
	api.HandleFunc("POST /api/users", s.adminOnly(s.writable(s.handleCreateUser)))
	api.HandleFunc("PUT /api/users/{id}", s.adminOnly(s.writable(s.handleUpdateUser)))
	api.HandleFunc("DELETE /api/users/{id}", s.adminOnly(s.writable(s.handleDeleteUser)))
	api.HandleFunc("POST /api/users/{id}/password", s.adminOnly(s.writable(s.handleResetUserPassword)))

	// API keys (per-user; admin tier gated in-handler).
	api.HandleFunc("GET /api/apikeys", s.protected(s.handleListAPIKeys))
	api.HandleFunc("POST /api/apikeys", s.writable(s.handleCreateAPIKey))
	api.HandleFunc("DELETE /api/apikeys/{id}", s.writable(s.handleDeleteAPIKey))
	api.HandleFunc("POST /api/apikeys/{id}/rotate", s.writable(s.handleRotateAPIKey))

	mux.Handle("/api/", s.chain(api))

	// WebSocket demo endpoint (cookie or ?token= auth — see extract()).
	mux.Handle("GET /ws", s.chain(http.HandlerFunc(s.handleWS)))

	// Everything else: the SPA. In dev, proxy to `next dev` for HMR;
	// in production, serve the embedded static export.
	root, err := s.rootHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", s.chain(root))
	return mux, nil
}

// Run starts the HTTP server and blocks until ctx is canceled. The
// shutdown path gives in-flight requests 5s to finish, then returns.
func (s *Server) Run(ctx context.Context) error {
	handler, err := s.BuildHandler()
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(bindHost(s.cfg.Bind), fmt.Sprintf("%d", s.cfg.Port))
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	slog.Info("server running",
		"url", fmt.Sprintf("http://localhost:%d", s.cfg.Port),
		"version", buildinfo.Version,
		"devProxy", s.cfg.DevProxy,
	)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// bindHost maps the Bind setting to a listen address.
func bindHost(bind string) string {
	if bind == "all" {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

// --- Route wrapper shorthand ---

// public: no auth (bootstrap routes), IP-keyed rate limit.
func (s *Server) public(next http.HandlerFunc) http.HandlerFunc {
	return s.auth.Optional(rateLimit(s.limiter, clientIP, next))
}

// protected: auth required, user-keyed rate limit.
func (s *Server) protected(next http.HandlerFunc) http.HandlerFunc {
	return s.auth.Middleware(rateLimit(s.limiter, identityUserID, next))
}

// writable: protected + rejects read-only (actAs) callers. Wrap ALL
// mutating handlers with this.
func (s *Server) writable(next http.HandlerFunc) http.HandlerFunc {
	return s.protected(auth.RequireWritable(next))
}

// adminOnly: protected + admin role (session admin or admin-tier key).
func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return s.protected(auth.RequireAdmin(next))
}

// userID helpers used as rate-limit keys.
func identityUserID(r *http.Request) string {
	if ident, ok := auth.FromContext(r.Context()); ok {
		if ident.UserID != "" {
			return "u:" + ident.UserID
		}
	}
	return "ip:" + clientIP(r)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
