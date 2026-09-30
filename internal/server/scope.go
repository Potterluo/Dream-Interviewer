package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Potterluo/dream-interviewer/internal/auth"
)

// scope.go decides WHOSE rows a list/aggregate request may see.
//
// The product rule (docs/API.md §6): a regular user sees only their own
// rows; an admin may widen the scope to one user or to everyone. That is
// the only place ownership is relaxed, and it is deliberately explicit:
//
//   - A non-admin asking for another user's rows, or for everything, gets
//     **403**. The template's original behaviour was to silently ignore
//     `?all=true`, which makes an escalation attempt indistinguishable from
//     an empty result set. Being loud costs nothing here and leaves a
//     usable audit trail in the request log.
//   - Single-row routes never use this: they keep the template's
//     404-not-403 rule so ids cannot be probed (see loadOwnedInterview).
//
// The store's "" = "all owners" convention is preserved, so the scope
// resolution stays in one place instead of leaking into every handler.

// scopeAll is the store-level owner filter meaning "no filter".
const scopeAll = ""

// resolveScope returns the owner filter for the request. It writes the
// error response itself and returns ok=false when the caller may not have
// what it asked for.
func (s *Server) resolveScope(w http.ResponseWriter, r *http.Request) (userID string, ok bool) {
	ident, _ := auth.FromContext(r.Context())
	q := r.URL.Query()
	wantsAll := q.Get("all") == "true"
	wantsUser := strings.TrimSpace(q.Get("userId"))

	// The common case: no scope parameters, so the caller sees their own.
	if !wantsAll && wantsUser == "" {
		return ident.UserID, true
	}

	if !ident.IsAdmin() {
		// The bare literal, deliberately: docs/API.md §6 publishes
		// `{"ok":false,"error":"forbidden"}` so a client can branch on one
		// stable value. The human-readable reason belongs in the log, not in
		// a machine-readable field — mixing the two is what produced a
		// near-miss where a listener matching "forbidden" only worked
		// because it was a substring.
		slog.Info("forbidden scope widening",
			"user", ident.UserID, "role", ident.Role,
			"all", wantsAll, "wantsUserId", wantsUser, "path", r.URL.Path)
		writeError(w, http.StatusForbidden, "forbidden")
		return "", false
	}

	if wantsUser != "" {
		// An admin may of course look at their own rows this way too.
		return wantsUser, true
	}
	return scopeAll, true
}

// requireAdmin re-checks the role for admin handlers. The route table is the
// authority (`s.adminOnly`), but a handler registered behind the wrong
// wrapper must still not leak or mutate data.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	ident, ok := auth.FromContext(r.Context())
	if !ok || !ident.IsAdmin() {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

// errNotAuthorized is the sentinel for scope/role refusals inside helpers
// that cannot write a response themselves.
var errNotAuthorized = errors.New("forbidden")
