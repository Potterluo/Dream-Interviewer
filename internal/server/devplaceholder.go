package server

import "net/http"

// devPlaceholder is served when the binary was built before
// `make build-web` filled internal/server/dist — i.e. the backend is
// fine, there is just no UI embedded yet. It tells the developer the
// two ways forward instead of 404ing.
func devPlaceholderHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>dream-interviewer</title>
<style>body{font-family:system-ui;display:flex;min-height:100vh;align-items:center;justify-content:center;background:#09090b;color:#e4e4e7}
code{background:#27272a;padding:2px 8px;border-radius:6px}div{text-align:center;line-height:2}</style></head>
<body><div><h1>dream-interviewer backend is running</h1>
<p>API: <code>GET /api/status</code> &middot; Health: <code>GET /healthz</code></p>
<p>Build the UI with <code>make build-web</code> and rebuild the binary,<br>
or run <code>pnpm dev</code> in web/ with <code>APP_DEV_PROXY=http://localhost:3000</code>.</p>
</div></body></html>`))
	})
}
