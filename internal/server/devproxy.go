package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// newDevProxy reverse-proxies everything to the Next.js dev server.
// Used only when APP_DEV_PROXY is set (see rootHandler).
//
// httputil.ReverseProxy also transparently handles the WebSocket upgrade
// that Next's HMR websocket needs, so `next dev` behaves exactly as if
// the browser had connected to :3000 directly — while /api/* and /ws
// still hit THIS process, keeping sessions same-origin.
func newDevProxy(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	orig := proxy.Director
	proxy.Director = func(req *http.Request) {
		orig(req)
		req.Host = target.Host // Next dev serves on localhost:3000; keep the Host honest
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<body style="font-family:system-ui;background:#09090b;color:#e4e4e7;padding:3rem">
<h2>dev server not reachable</h2><p>Start it with <code>pnpm dev</code> in web/ (target: ` +
			target.String() + `)</p></body>`))
	}
	return proxy
}
