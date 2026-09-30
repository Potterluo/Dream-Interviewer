// Package auth resolves an HTTP request to a user identity. It supports
// two credential types:
//
//   - cookie session ("app_session"): set by /api/login, validated
//     against the web_sessions table; used by the web UI
//   - Bearer API key ("sk_..."): validated against the apikeys table;
//     used by programmatic clients
//
// Both paths funnel into the same Identity struct stamped onto the
// request context. There is no anonymous fallback — API routes without
// valid credentials get 401 (the few pre-login routes use Optional).
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/store"
)

// SessionCookieName is the cookie that backs the web UI's login state.
const SessionCookieName = "app_session"

// SessionTTL is how long a freshly issued login cookie is valid.
const SessionTTL = 30 * 24 * time.Hour

// Auth method values carried on Identity.
const (
	AuthSession = "session"
	AuthAPIKey  = "apikey"
)

// Identity is the resolved caller for one request.
type Identity struct {
	UserID string
	Role   string

	// AuthMethod is "session" or "apikey".
	AuthMethod string

	// APIKeyID / APIKeyType are set when AuthMethod == "apikey".
	APIKeyID   string
	APIKeyType string

	// ActAsUserID is non-empty when an admin browses another user's
	// resources read-only via ?actAs=<userID>. Mutating handlers MUST
	// reject when ReadOnly() is true (RequireWritable does this).
	ActAsUserID string
}

// EffectiveUserID is who we read data for: the impersonated user during
// actAs, the caller themselves otherwise.
func (i Identity) EffectiveUserID() string {
	if i.ActAsUserID != "" {
		return i.ActAsUserID
	}
	return i.UserID
}

// IsActingAs reports whether admin impersonation is active.
func (i Identity) IsActingAs() bool {
	return i.ActAsUserID != "" && i.ActAsUserID != i.UserID
}

// ReadOnly reports whether mutating endpoints must reject this request.
func (i Identity) ReadOnly() bool { return i.IsActingAs() }

// IsAdmin answers "may this caller hit admin endpoints?".
//
// Two conditions, and BOTH are needed:
//
//  1. The owner's CURRENT role is admin. For a session and for an API key
//     alike, Role is re-read from the database on every request
//     (resolveSession / resolveAPIKey), so demoting an account takes effect
//     immediately and no credential outlives the role that justified it.
//  2. For an API key, the key's own tier is admin as well. A lower-tier key
//     issued by an admin stays deliberately narrow — a key may therefore
//     never WIDEN the access its owner has.
//
// Checking only the key tier was a real privilege-escalation bug: a demoted
// admin's admin-tier key kept full admin and could re-promote its owner,
// because revocation by role never reached the key path. Disabling the
// account did revoke it, which is what made the hole easy to miss.
func (i Identity) IsAdmin() bool {
	if i.Role != store.RoleAdmin {
		return false
	}
	if i.AuthMethod == AuthAPIKey && i.APIKeyType != store.APIKeyTypeAdmin {
		return false
	}
	return true
}

type identityKey struct{}

// WithIdentity stamps the resolved identity onto ctx so handlers can
// read it without re-validating credentials.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// FromContext returns the identity stamped by Middleware. ok is false
// when auth never ran — a route registered outside the middleware.
func FromContext(ctx context.Context) (Identity, bool) {
	if ctx == nil {
		return Identity{}, false
	}
	v, ok := ctx.Value(identityKey{}).(Identity)
	return v, ok
}

// Resolver validates credentials against the store and issues sessions.
type Resolver struct {
	store store.Store
}

// NewResolver returns a resolver bound to the store.
func NewResolver(st store.Store) *Resolver {
	return &Resolver{store: st}
}

// IssueSession creates a web session for userID and returns the cookie
// to write on the response.
func (r *Resolver) IssueSession(ctx context.Context, userID string) (*http.Cookie, error) {
	sid, err := newRandomHex(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := &store.WebSession{SID: sid, UserID: userID, CreatedAt: now, ExpiresAt: now.Add(SessionTTL)}
	if err := r.store.CreateWebSession(ctx, sess); err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure is set by the server layer when APP_COOKIE_SECURE is on
		// (HTTPS-only deployments); see server.overrideCookie.
		Expires: sess.ExpiresAt,
	}, nil
}

// RevokeSession drops a session (logout).
func (r *Resolver) RevokeSession(ctx context.Context, sid string) error {
	return r.store.DeleteWebSession(ctx, sid)
}

// ResolveSession turns a cookie SID into an Identity.
func (r *Resolver) ResolveSession(ctx context.Context, sid string) (Identity, error) {
	if sid == "" {
		return Identity{}, ErrUnauthorized
	}
	sess, err := r.store.GetWebSession(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Identity{}, ErrUnauthorized
		}
		return Identity{}, err
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = r.store.DeleteWebSession(ctx, sid)
		return Identity{}, ErrUnauthorized
	}
	user, err := r.store.GetUser(ctx, sess.UserID)
	if err != nil || user.Status != store.StatusActive {
		return Identity{}, ErrUnauthorized
	}
	return Identity{UserID: user.ID, Role: user.Role, AuthMethod: AuthSession}, nil
}

// ResolveBearer turns a Bearer API key into an Identity.
func (r *Resolver) ResolveBearer(ctx context.Context, token string) (Identity, error) {
	res, err := LookupAPIKeyByToken(ctx, r.store, token)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, store.ErrNotFound) {
			return Identity{}, ErrUnauthorized
		}
		return Identity{}, err
	}
	return Identity{
		UserID:     res.User.ID,
		Role:       res.User.Role,
		AuthMethod: AuthAPIKey,
		APIKeyID:   res.Key.ID,
		APIKeyType: res.Key.Type,
	}, nil
}

// ErrUnauthorized is returned when no valid credential is present.
var ErrUnauthorized = errors.New("unauthorized")

// extract pulls the bearer token (if any) and session SID (if any) from
// a request. A `?token=` query param is also accepted as a bearer, but
// ONLY on the allow-listed paths that genuinely cannot set headers —
// EventSource (SSE) has no header API and <a download> links don't carry
// one either. Everywhere else the fallback is denied: tokens in URLs
// leak via Referer, browser history, and access logs.
func extract(r *http.Request) (bearer, sid string) {
	if c, err := r.Cookie(SessionCookieName); err == nil {
		sid = c.Value
	}
	if h := r.Header.Get("Authorization"); h != "" {
		if t := strings.TrimPrefix(h, "Bearer "); t != h {
			bearer = t
		}
	} else if t := r.URL.Query().Get("token"); t != "" && queryTokenAllowed(r) {
		bearer = t
	}
	return bearer, sid
}

// queryTokenAllowed gates the `?token=` fallback. Only long-lived GET
// streams and file downloads may use it — never a write path.
func queryTokenAllowed(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	p := r.URL.Path
	if p == "/api/events" || p == "/ws" {
		return true
	}
	// File downloads: <a href> links and HTTP clients can't easily add
	// headers; the path shape keeps this scoped to one resource type.
	if strings.HasPrefix(p, "/api/files/") {
		return true
	}
	return false
}

// Middleware enforces auth on the wrapped route: 401 on missing or
// invalid credentials, then resolves ?actAs= for admins.
func (r *Resolver) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ident, err := r.resolve(req)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		req = req.WithContext(WithIdentity(req.Context(), ident))
		next(w, req)
	}
}

// Optional resolves credentials when present and lets unauthenticated
// requests through. Used by the bootstrap routes (status, onboard,
// login) that must work before any user exists.
func (r *Resolver) Optional(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if ident, err := r.resolve(req); err == nil {
			req = req.WithContext(WithIdentity(req.Context(), ident))
		}
		next(w, req)
	}
}

func (r *Resolver) resolve(req *http.Request) (Identity, error) {
	bearer, sid := extract(req)
	if sid != "" {
		if ident, err := r.ResolveSession(req.Context(), sid); err == nil {
			return applyActAs(req, ident), nil
		}
	}
	if bearer != "" {
		if ident, err := r.ResolveBearer(req.Context(), bearer); err == nil {
			return applyActAs(req, ident), nil
		}
	}
	return Identity{}, ErrUnauthorized
}

// applyActAs honors ?actAs=<userID> for admin SESSIONS only. API keys
// never impersonate — that would defeat their scoping. The resulting
// identity is read-only; RequireWritable blocks mutations.
func applyActAs(req *http.Request, ident Identity) Identity {
	act := req.URL.Query().Get("actAs")
	if act == "" || ident.AuthMethod != AuthSession || ident.Role != store.RoleAdmin {
		return ident
	}
	ident.ActAsUserID = act
	return ident
}

// RequireAdmin 403s non-admin callers. Wrap around Middleware.
//
// The message is the single literal "forbidden" (see docs/API.md §6) so a
// client can branch on one stable value for every permission refusal,
// whether it came from this gate or from an in-handler role/scope check.
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ident, ok := FromContext(req.Context())
		if !ok {
			writeUnauthorized(w)
			return
		}
		if !ident.IsAdmin() {
			writeForbidden(w, "forbidden")
			return
		}
		next(w, req)
	}
}

// RequireWritable rejects requests where Identity.ReadOnly() is true —
// i.e. an admin acting as another user. Wrap every mutating handler.
func RequireWritable(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ident, ok := FromContext(req.Context())
		if !ok {
			writeUnauthorized(w)
			return
		}
		if ident.ReadOnly() {
			writeForbidden(w, "read-only: cannot mutate while acting as another user")
			return
		}
		next(w, req)
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"ok":false,"error":"unauthorized"}`))
}

func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"ok":false,"error":"` + msg + `"}`))
}

// newRandomHex returns 2*n hex characters of crypto/rand entropy.
func newRandomHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
