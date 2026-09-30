package server

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/buildinfo"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// handlers_auth.go: bootstrap + session lifecycle.
//
// Bootstrap flow (mirrors a real product's first-run experience):
//
//	GET /api/status  — safe to call before login; tells the frontend
//	                   whether the instance has an admin yet
//	POST /api/onboard— creates the FIRST user as admin (409 afterwards)
//	POST /api/login  — username-or-email + password → session cookie
//
// The frontend's AuthGuard drives the same three states.

// userDTO is the public form of a user. The password hash NEVER leaves
// the backend — this DTO is the only shape handlers emit.
type userDTO struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toUserDTO(u *store.User) userDTO {
	return userDTO{
		ID: u.ID, Username: u.Username, Email: u.Email,
		DisplayName: u.DisplayName, Role: u.Role, Status: u.Status,
		CreatedAt: u.CreatedAt,
	}
}

// handleStatus is the unauthenticated probe the AuthGuard calls on every
// page load: configured=false means "run the onboarding wizard".
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.CountUsers(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	resp := map[string]any{
		"ok":         true,
		"configured": count > 0,
		"version":    buildinfo.Version,
	}
	// If the caller happens to carry a valid session, include who they
	// are so the frontend can skip a second round-trip.
	if ident, ok := auth.FromContext(r.Context()); ok {
		resp["authenticated"] = true
		if u, err := s.store.GetUser(r.Context(), ident.UserID); err == nil {
			resp["user"] = toUserDTO(u)
		}
	} else {
		resp["authenticated"] = false
	}
	writeOK(w, resp)
}

// handleOnboard creates the first user — as admin — and logs them in.
// It hard-fails with 409 once any user exists; normal user creation goes
// through POST /api/users (admin-only) afterwards.
func (s *Server) handleOnboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if msg := validateNewAccount(req.Username, req.Email, req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	count, err := s.store.CountUsers(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "already onboarded")
		return
	}
	u, err := s.createUser(r, req.Username, req.Email, req.Password, req.DisplayName, store.RoleAdmin)
	if err != nil {
		storeError(w, err)
		return
	}
	s.loginResponse(w, r, u)
}

// handleLogin authenticates a username-or-email + password and sets the
// session cookie. Every failure path returns the same message — never
// reveal whether the account exists.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUserByLogin(r.Context(), strings.TrimSpace(req.Login))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if u.Status != store.StatusActive {
		writeError(w, http.StatusForbidden, "account disabled")
		return
	}
	s.loginResponse(w, r, u)
}

// loginResponse issues the session cookie and answers with the user.
func (s *Server) loginResponse(w http.ResponseWriter, r *http.Request, u *store.User) {
	cookie, err := s.auth.IssueSession(r.Context(), u.ID)
	if err != nil {
		storeError(w, err)
		return
	}
	http.SetCookie(w, s.secureCookie(cookie))
	writeOK(w, map[string]any{"user": toUserDTO(u)})
}

// secureCookie applies the deployment-wide Secure flag to a session
// cookie. Secure=true makes browsers refuse to send it over plain HTTP —
// turn APP_COOKIE_SECURE on in production (HTTPS) and off for localhost.
func (s *Server) secureCookie(c *http.Cookie) *http.Cookie {
	c.Secure = s.cfg.CookieSecure
	return c
}

// handleLogout revokes the session server-side and clears the cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookieName); err == nil {
		_ = s.auth.RevokeSession(r.Context(), c.Value)
	}
	http.SetCookie(w, s.secureCookie(&http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}))
	writeOK(w, nil)
}

// handleMe returns the caller (and actAs context when impersonating).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	u, err := s.store.GetUser(r.Context(), ident.EffectiveUserID())
	if err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"user":        toUserDTO(u),
		"authMethod":  ident.AuthMethod,
		"actAsUserId": ident.ActAsUserID,
		"readOnly":    ident.ReadOnly(),
	})
}

// handleUpdateMe changes the caller's display name. Username/email/role
// changes are NOT self-service (username+email are identity anchors);
// role/status go through admin endpoints only.
func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUser(r.Context(), ident.UserID)
	if err != nil {
		storeError(w, err)
		return
	}
	u.DisplayName = strings.TrimSpace(req.DisplayName)
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"user": toUserDTO(u)})
}

// handleChangeMyPassword verifies the old password, then rotates the
// hash. Always require the old password here — a hijacked session must
// not be able to lock the real owner out silently.
func (s *Server) handleChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUser(r.Context(), ident.UserID)
	if err != nil {
		storeError(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		writeError(w, http.StatusForbidden, "current password is incorrect")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		storeError(w, err)
		return
	}
	u.PasswordHash = hash
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, nil)
}

// --- Validation + shared user creation ---

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// validateNewAccount returns "" when the fields are acceptable, else a
// user-facing message.
func validateNewAccount(username, email, password string) string {
	if !usernameRe.MatchString(username) {
		return "username must be 3-32 characters (letters, digits, _ or -)"
	}
	if !emailRe.MatchString(email) {
		return "a valid email is required"
	}
	if len(password) < 8 {
		return "password must be at least 8 characters"
	}
	return ""
}

// createUser hashes the password and persists a new account. The only
// place new User rows are born (onboard + admin create-user both funnel
// through here so validation and defaults can't drift).
func (s *Server) createUser(r *http.Request, username, email, password, displayName, role string) (*store.User, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	id, err := randomID("u_")
	if err != nil {
		return nil, err
	}
	u := &store.User{
		ID:           id,
		Username:     strings.TrimSpace(username),
		Email:        strings.TrimSpace(strings.ToLower(email)),
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(displayName),
		Role:         role,
		Status:       store.StatusActive,
	}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		return nil, err
	}
	return u, nil
}
