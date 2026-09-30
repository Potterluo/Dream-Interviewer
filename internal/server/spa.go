package server

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

// rootHandler returns the handler for everything that is not /api/* or
// /ws: the single-page app. Development proxies to `next dev` (HMR);
// production serves the embedded static export.
func (s *Server) rootHandler() (http.Handler, error) {
	if s.cfg.DevProxy != "" {
		target, err := url.Parse(s.cfg.DevProxy)
		if err != nil {
			return nil, err
		}
		// httputil.ReverseProxy handles the WebSocket upgrade Next's dev
		// server needs for HMR, so the browser only ever talks to THIS
		// server and cookies stay same-origin.
		return newDevProxy(target), nil
	}
	root, err := fs.Sub(webFS, "dist")
	if err != nil {
		return nil, err
	}
	// Before the first `make build-web`, dist/ holds only .gitkeep —
	// serve the instructions placeholder instead of 404ing every route.
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return devPlaceholderHandler(), nil
	}
	return spaHandler{fs: root}, nil
}

// spaHandler serves the embedded Next.js static export with SPA-style
// fallback. Resolution order for a request path:
//
//  1. exact file          /logo.png            → logo.png
//  2. directory index     /dashboard/          → dashboard/index.html
//  3. flat page           /items/              → items.html (if exported so)
//  4. root fallback       /anything-else       → index.html (client routing)
//
// The frontend is exported with trailingSlash: true, so route URLs map
// to <route>/index.html and case 2 covers navigation; the later cases
// keep deep links and unknown paths inside the app.
type spaHandler struct {
	fs fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	fsPath := strings.TrimPrefix(path, "/")
	if fsPath == "" {
		fsPath = "."
	}

	// 1. Exact asset (js, css, images, fonts, RSC payloads).
	if f, err := h.fs.Open(fsPath); err == nil {
		stat, statErr := f.Stat()
		f.Close()
		if statErr == nil && !stat.IsDir() {
			http.ServeFileFS(w, r, h.fs, fsPath)
			return
		}
	}

	// 2. Directory index (<route>/index.html).
	indexPath := "index.html"
	if fsPath != "." {
		indexPath = fsPath + "/index.html"
	}
	if f, err := h.fs.Open(indexPath); err == nil {
		f.Close()
		http.ServeFileFS(w, r, h.fs, indexPath)
		return
	}

	// 3. Flat page (<route>.html).
	if fsPath != "." {
		if f, err := h.fs.Open(fsPath + ".html"); err == nil {
			f.Close()
			http.ServeFileFS(w, r, h.fs, fsPath+".html")
			return
		}
	}

	// 4. Root fallback — the client router takes over.
	http.ServeFileFS(w, r, h.fs, "index.html")
}
