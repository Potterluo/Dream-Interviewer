package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Potterluo/dream-interviewer/internal/store"
)

// respond.go: the ONE response shape and the ONE way to read request
// bodies. Every JSON endpoint in the template returns:
//
//	{"ok": true, ...}           on success
//	{"ok": false, "error": "..."}  on failure
//
// Keeping a single envelope means the frontend's apiFetch can branch on
// `ok` without per-endpoint parsers.

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeOK emits {"ok":true, ...fields}.
func writeOK(w http.ResponseWriter, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["ok"] = true
	writeJSON(w, http.StatusOK, fields)
}

// writeError emits {"ok":false,"error":msg} with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

// readJSON decodes a JSON request body into dst with a 1 MiB cap. The
// cap is a DoS floor, not a business rule — raise it per-endpoint if a
// domain needs bigger payloads.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return false
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "empty request body")
		return false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

// storeError maps store errors to HTTP responses with safe messages —
// internal details (SQL, DSN) must never reach the client.
func storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
