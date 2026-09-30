package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/events"
)

// events.go: server-sent events — the primary realtime channel for the
// web UI. One GET /api/events stream per browser tab; the hub fans item
// mutations out to the owning user's streams.
//
// Why SSE over WebSocket for push: it is plain HTTP (survives proxies,
// needs no upgrade), auto-reconnects in the browser for free, and the
// only thing the browser can't do is SEND on the stream — which is what
// POST endpoints are for. /ws exists for the cases that genuinely need
// bidirectional (see ws.go).

// handleEvents holds an SSE stream open, forwarding hub events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	// Proxies between the server and the browser must not buffer this
	// response (nginx: proxy_buffering off).
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sub := s.hub.Subscribe(ident.UserID)
	defer s.hub.Unsubscribe(sub)

	writeSSE := func(evt events.Event) bool {
		b, err := json.Marshal(evt)
		if err != nil {
			return true // skip this event; keep the stream alive
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return false // client gone
		}
		flusher.Flush()
		return true
	}

	// Hello frame lets the client confirm the stream is live (and that
	// auth worked) instead of waiting on an event that may never come.
	if !writeSSE(events.Event{Type: "hello"}) {
		return
	}

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-sub.Ch:
			if !ok {
				return
			}
			if !writeSSE(evt) {
				return
			}
		case <-ping.C:
			if !writeSSE(events.Event{Type: "ping"}) {
				return
			}
		}
	}
}
