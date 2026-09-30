package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"

	"github.com/gorilla/websocket"
)

// ws.go: WebSocket demo endpoint. SSE (events.go) covers server→client
// push for the web UI; use this when you need the client to push too
// (collaborative editing, chat, live queries).
//
// Tiny frame protocol, JSON in both directions:
//
//	server → client: {"type":"hello", "data":{"userId":"…"}}
//	                 {"type":"event", "data":{...hub event...}}
//	                 {"type":"pong"}
//	client → server: {"type":"ping"}
//
// Extend by adding request types with an id field and answering with
// {"type":"res","id":…} — the same req/res/event envelope pattern used
// by production apps.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// gorilla's default CheckOrigin rejects cross-origin upgrades; the
	// SPA is same-origin, and script clients send no Origin header.
	ident, err := s.auth.ResolveBearer(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		if c, cerr := r.Cookie(auth.SessionCookieName); cerr == nil {
			ident, err = s.auth.ResolveSession(r.Context(), c.Value)
		}
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // upgrader already wrote the HTTP error
	}
	defer conn.Close()

	sub := s.hub.Subscribe(ident.UserID)
	defer s.hub.Unsubscribe(sub)

	writeJSONFrame := func(v any) error {
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)
	}

	if err := writeJSONFrame(map[string]any{
		"type": "hello", "data": map[string]string{"userId": ident.UserID},
	}); err != nil {
		return
	}

	// Server pings keep NATs and proxies from idling the connection out;
	// pong handling below mirrors the app-level ping too.
	conn.SetReadLimit(1 << 20)
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}()

	errCh := make(chan error, 2)
	// Reader: client frames.
	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			var frame struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &frame) != nil || frame.Type != "ping" {
				_ = writeJSONFrame(map[string]any{"type": "error", "data": "unsupported frame"})
				continue
			}
			if writeJSONFrame(map[string]any{"type": "pong"}) != nil {
				errCh <- err
				return
			}
		}
	}()
	// Forwarder: hub events → this socket.
	go func() {
		for evt := range sub.Ch {
			if writeJSONFrame(map[string]any{"type": "event", "data": evt}) != nil {
				errCh <- nil
				return
			}
		}
		errCh <- nil
	}()
	// First side to fail ends the handler; the defers close the socket
	// and unsubscribe from the hub, which ends the other goroutine.
	<-errCh
}

// upgrader keeps the library's conservative same-origin check. Add
// explicit origins here ONLY if you split the frontend to another domain.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}
