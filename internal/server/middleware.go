package server

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// middleware.go: the cross-cutting wrappers. Everything routes through
// s.chain (logging → recovery → security headers); rate limiting is a
// per-route concern applied by public/protected.

// chain wraps h with the middleware every request gets.
func (s *Server) chain(h http.Handler) http.Handler {
	return requestLogger(recoverPanics(secureHeaders(h)))
}

// statusRecorder captures the status code for logging and forwards the
// optional interfaces handlers legitimately need through the wrapper:
// Flush for SSE, Hijack for protocol upgrades (WebSockets — including
// `next dev`'s HMR socket when APP_DEV_PROXY is active).
type statusRecorder struct {
	http.ResponseWriter
	status   int
	flusher  http.Flusher
	hijacker http.Hijacker
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher (only attached when supported) so type
// assertions on the wrapped writer keep succeeding for SSE.
func (r *statusRecorder) Flush() {
	if r.flusher != nil {
		r.flusher.Flush()
	}
}

// Hijack implements http.Hijacker (same conditional attach).
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if r.hijacker != nil {
		return r.hijacker.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

// requestLogger writes one line per request. Health probes are skipped
// — they fire every few seconds and would be the only log line most
// deployments ever produce.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/livez" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		if f, ok := w.(http.Flusher); ok {
			rec.flusher = f
		}
		if h, ok := w.(http.Hijacker); ok {
			rec.hijacker = h
		}
		next.ServeHTTP(rec, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).Round(time.Millisecond).String(),
			"ip", clientIP(r),
		)
	})
}

// recoverPanics turns a panicking handler into a 500 instead of a dead
// connection (and keeps the process alive — one bad request must never
// take down the server).
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"method", r.Method, "path", r.URL.Path,
					"err", err, "stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ok":false,"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// secureHeaders sets conservative defaults on every response. There is
// deliberately NO CORS header: the SPA is same-origin (embedded in this
// binary). If you split the frontend out, add an explicit origin
// allow-list — never `*` together with credentials.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// --- Rate limiting (per-key sliding window, in memory) ---

type rateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time // key → request timestamps
	rpm     int                    // requests per minute (0 = unlimited)
	window  time.Duration
}

func newRateLimiter(rpm int) *rateLimiter {
	if rpm <= 0 {
		return &rateLimiter{rpm: 0}
	}
	return &rateLimiter{
		windows: make(map[string][]time.Time),
		rpm:     rpm,
		window:  time.Minute,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	if rl.rpm <= 0 {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	ts := rl.windows[key]
	start := 0
	for start < len(ts) && ts[start].Before(cutoff) {
		start++
	}
	ts = ts[start:]
	if len(ts) >= rl.rpm {
		rl.windows[key] = ts
		return false
	}
	rl.windows[key] = append(ts, now)
	return true
}

// rateLimit keys requests by keyFunc (user id when authenticated, IP
// otherwise) and answers 429 once the per-minute budget is spent.
func rateLimit(rl *rateLimiter, keyFunc func(r *http.Request) string, next http.HandlerFunc) http.HandlerFunc {
	if rl == nil || rl.rpm <= 0 {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(keyFunc(r)) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded — try again shortly")
			return
		}
		next(w, r)
	}
}
