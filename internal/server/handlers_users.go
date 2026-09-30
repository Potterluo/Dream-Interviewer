package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/interview"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// Status constants mirrored from the interview domain, so this file does not
// need to import the domain just to compare a string.
const (
	interviewStatusCompleted  = "completed"
	interviewStatusInProgress = "in_progress"
	interviewStatusAborted    = "aborted"
)

// handlers_users.go: admin-only account management. Everything here is
// gated by s.adminOnly in the route table; the handlers themselves only
// worry about business rules.

// handleListUsers returns every account. Small installs can list all;
// add pagination when your user table outgrows one screen.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for i := range users {
		out = append(out, toUserDTO(&users[i]))
	}
	writeOK(w, map[string]any{"users": out})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
		Role        string `json:"role"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if msg := validateNewAccount(req.Username, req.Email, req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	role := store.RoleUser
	if req.Role != "" {
		if req.Role != store.RoleAdmin && req.Role != store.RoleUser {
			writeError(w, http.StatusBadRequest, "role must be admin or user")
			return
		}
		role = req.Role
	}
	u, err := s.createUser(r, req.Username, req.Email, req.Password, req.DisplayName, role)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "username or email already exists")
			return
		}
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "user": toUserDTO(u)})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		DisplayName *string `json:"displayName"`
		Role        *string `json:"role"`
		Status      *string `json:"status"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUser(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	if req.DisplayName != nil {
		u.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Role != nil {
		if *req.Role != store.RoleAdmin && *req.Role != store.RoleUser {
			writeError(w, http.StatusBadRequest, "role must be admin or user")
			return
		}
		if u.ID == callerID(r) && *req.Role != store.RoleAdmin {
			writeError(w, http.StatusBadRequest, "cannot demote yourself")
			return
		}
		u.Role = *req.Role
	}
	if req.Status != nil {
		if *req.Status != store.StatusActive && *req.Status != store.StatusDisabled {
			writeError(w, http.StatusBadRequest, "status must be active or disabled")
			return
		}
		if u.ID == callerID(r) && *req.Status != store.StatusActive {
			writeError(w, http.StatusBadRequest, "cannot disable yourself")
			return
		}
		u.Status = *req.Status
	}
	// Never strand the install without a working admin.
	if u.Role == store.RoleAdmin && u.Status == store.StatusDisabled {
		count, err := s.store.CountUsers(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		// Coarse guard: with more than one account, some other admin can
		// undo a mistake; refuse only when it's the ONLY account.
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot disable the only admin")
			return
		}
	}
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"user": toUserDTO(u)})
}

// handleDeleteUser removes the account and, via the store's cascade, its
// sessions, API keys, interviews, interview turns and uploaded files.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == callerID(r) {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	u, err := s.store.GetUser(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	if u.Role == store.RoleAdmin {
		count, err := s.store.CountUsers(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot delete the only admin")
			return
		}
	}
	// Collect the blob ids BEFORE the rows go away: the store cascade drops
	// the file metadata, and without this step every uploaded byte would be
	// left on disk forever, unreachable by any route.
	files, ferr := s.store.ListFileRecords(r.Context(), id)
	if ferr != nil {
		storeError(w, ferr)
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	// Best-effort, like single-file delete: a leaked blob is a janitor
	// problem, whereas a row with no file 404s cleanly on download.
	for i := range files {
		_ = os.Remove(filepath.Join(s.filesDir(), files[i].ID))
	}
	writeOK(w, nil)
}

// handleResetUserPassword sets a new password WITHOUT knowing the old
// one — the admin escape hatch for "user forgot everything". The user's
// existing sessions stay valid (they were authenticated with the old
// credential); delete them here if you want stricter semantics.
func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	u, err := s.store.GetUser(r.Context(), id)
	if err != nil {
		storeError(w, err)
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

// --- admin: per-user interview overview -------------------------------------

// userOverviewDTO is the summary an admin sees next to an account: how much
// that person has used the product and how they are doing. It is the entry
// point for "show me this user's interviews".
type userOverviewDTO struct {
	User                    userDTO      `json:"user"`
	Total                   int          `json:"total"`
	Completed               int          `json:"completed"`
	InProgress              int          `json:"inProgress"`
	Aborted                 int          `json:"aborted"`
	AvgScore                float64      `json:"avgScore"`
	BestScore               float64      `json:"bestScore"`
	LastActivityAt          *time.Time   `json:"lastActivityAt"`
	RecommendationBreakdown []labelCount `json:"recommendationBreakdown"`
}

func (s *Server) handleUserOverview(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	u, err := s.store.GetUser(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	summaries, err := s.store.ListInterviewSummaries(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}

	out := userOverviewDTO{User: toUserDTO(u), RecommendationBreakdown: []labelCount{}}
	scoreSum, scored := 0.0, 0
	recCounts := map[string]int{}
	var last *time.Time
	touch := func(t time.Time) {
		u := t.UTC()
		if last == nil || u.After(*last) {
			last = &u
		}
	}
	for i := range summaries {
		sm := &summaries[i]
		out.Total++
		switch sm.Status {
		case interviewStatusCompleted:
			out.Completed++
			scoreSum += sm.OverallScore
			scored++
			if sm.OverallScore > out.BestScore {
				out.BestScore = sm.OverallScore
			}
			if sm.Recommendation != "" {
				recCounts[interview.LabelOf("recommendation", sm.Recommendation)]++
			}
		case interviewStatusInProgress:
			out.InProgress++
		case interviewStatusAborted:
			out.Aborted++
		}
		touch(sm.CreatedAt)
		if sm.CompletedAt != nil {
			touch(*sm.CompletedAt)
		}
	}
	if scored > 0 {
		out.AvgScore = roundTo1(scoreSum / float64(scored))
	}
	out.LastActivityAt = last
	out.RecommendationBreakdown = orderedLabelCounts(recCounts,
		[]string{interview.RecStrongHire, interview.RecHire, interview.RecMaybe, interview.RecNoHire},
		"recommendation")

	writeOK(w, map[string]any{"overview": out})
}

// callerID is shorthand for the authenticated user's own id.
func callerID(r *http.Request) string {
	ident, _ := auth.FromContext(r.Context())
	return ident.UserID
}

// isUniqueViolation reports whether err is a UNIQUE constraint failure
// (duplicate username/email). The store layer normalizes both drivers'
// errors into store.ErrDuplicate; handlers only see the semantic form.
func isUniqueViolation(err error) bool {
	return errors.Is(err, store.ErrDuplicate)
}
