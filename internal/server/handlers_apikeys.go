package server

import (
	"net/http"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// handlers_apikeys.go: per-user programmatic credentials. Rules:
//
//   - plaintext token is returned exactly once (create/rotate)
//   - list responses carry only the masked prefix
//   - type=admin keys require an admin caller (in-handler policy; the
//     route itself only requires being logged in)
//   - users manage their own keys; admins may manage anyone's

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	recs, err := s.store.ListAPIKeys(r.Context(), ident.UserID)
	if err != nil {
		storeError(w, err)
		return
	}
	out := make([]*auth.APIKey, 0, len(recs))
	for i := range recs {
		out = append(out, auth.MaskedAPIKey(&recs[i]))
	}
	writeOK(w, map[string]any{"apiKeys": out})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	keyType := store.APIKeyTypeUser
	if req.Type != "" {
		if req.Type != store.APIKeyTypeUser && req.Type != store.APIKeyTypeAdmin {
			writeError(w, http.StatusBadRequest, "type must be user or admin")
			return
		}
		keyType = req.Type
	}
	if keyType == store.APIKeyTypeAdmin && !ident.IsAdmin() {
		writeError(w, http.StatusForbidden, "admin role required to issue admin keys")
		return
	}
	key, err := auth.CreateAPIKey(r.Context(), s.store, ident.UserID, req.Name, keyType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "apiKey": key})
}

// loadOwnedAPIKey enforces "own key or admin" with a 404 for strangers.
func (s *Server) loadOwnedAPIKey(w http.ResponseWriter, r *http.Request) (*store.APIKey, bool) {
	ident, _ := auth.FromContext(r.Context())
	rec, err := s.store.GetAPIKey(r.Context(), r.PathValue("id"))
	if err != nil {
		storeError(w, err)
		return nil, false
	}
	if rec.UserID != ident.UserID && !ident.IsAdmin() {
		writeError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return rec, true
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.loadOwnedAPIKey(w, r); !ok {
		return
	}
	if err := s.store.DeleteAPIKey(r.Context(), r.PathValue("id")); err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, nil)
}

func (s *Server) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.loadOwnedAPIKey(w, r); !ok {
		return
	}
	token, err := auth.RotateAPIKey(r.Context(), s.store, r.PathValue("id"))
	if err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"key": token})
}
