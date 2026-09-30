// Package events implements a tiny in-process pub/sub hub that fans
// events out to SSE subscribers and WebSocket clients.
//
// Scope: one process, in memory. If you later need multi-process
// fan-out (app behind a load balancer), the seam to change is
// Hub.Publish — bridge it to Redis pub/sub or Streams and re-publish
// locally. Nothing else in the codebase needs to change.
package events

import (
	"sync"
)

// Event is one broadcast payload, marshaled to JSON exactly as it is
// written onto the wire.
type Event struct {
	Type string `json:"type"` // e.g. "item.created", "ping"
	Data any    `json:"data,omitempty"`
}

// Subscriber is one live client stream. Handlers own the lifecycle:
// Subscribe on connect, defer Unsubscribe, range over Ch.
type Subscriber struct {
	// UserID stamps who is listening so PublishTo can target one user.
	UserID string
	Ch     chan Event
}

// Hub is the fan-out point. Zero-value is NOT usable; construct with New.
type Hub struct {
	mu   sync.RWMutex
	subs map[*Subscriber]struct{}
}

// New returns an empty hub.
func New() *Hub {
	return &Hub{subs: make(map[*Subscriber]struct{})}
}

// Subscribe registers a listener. The channel is buffered so a slow
// reader doesn't block publishers; if the buffer fills the subscriber
// is dropped (better one stale tab than a blocked server).
func (h *Hub) Subscribe(userID string) *Subscriber {
	s := &Subscriber{UserID: userID, Ch: make(chan Event, 16)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe removes and closes a subscriber.
func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.Ch)
	}
	h.mu.Unlock()
}

// Publish broadcasts an event to every subscriber.
func (h *Hub) Publish(evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		deliver(s, evt)
	}
}

// PublishTo broadcasts to a single user's streams (their own data
// changes) — everyone else never sees it.
func (h *Hub) PublishTo(userID string, evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if s.UserID == userID {
			deliver(s, evt)
		}
	}
}

func deliver(s *Subscriber, evt Event) {
	select {
	case s.Ch <- evt:
	default:
		// Buffer full: the client stopped reading. Drop the event; the
		// stream's ping loop will notice a dead connection soon.
	}
}
