package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/interview"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// handlers_presets.go: the shared preset library.
//
// Division of ownership, which the UI mirrors:
//
//   - built-in presets are compiled into internal/interview and are
//     READ-ONLY. They are the curated product surface; an admin editing one
//     would either be lost on the next build or would turn the curated set
//     into data. Editing or deleting one is a 409, never a silent no-op.
//   - presets an admin creates live in the database, are visible to every
//     user, and can be edited or deleted by any admin.
//
// Everyone authenticated can READ the list (a user has to be able to pick
// one); only admins can change it.

// presetDTO is the wire shape of both kinds. `builtin` tells the UI which
// buttons to offer.
type presetDTO struct {
	ID            string   `json:"id"`
	Role          string   `json:"role"`
	Level         string   `json:"level"`
	InterviewType string   `json:"interviewType"`
	Difficulty    string   `json:"difficulty"`
	QuestionCount int      `json:"questionCount"`
	FocusAreas    []string `json:"focusAreas"`
	JDSample      string   `json:"jdSample"`
	Description   string   `json:"description"`
	Builtin       bool     `json:"builtin"`
	CreatedBy     string   `json:"createdBy"`
}

func builtinPresetDTO(p interview.Preset) presetDTO {
	return presetDTO{
		ID: p.ID, Role: p.Role, Level: p.Level, InterviewType: p.InterviewType,
		Difficulty: p.Difficulty, QuestionCount: p.QuestionCount,
		FocusAreas: strSliceOrEmpty(p.FocusAreas), JDSample: p.JDSample,
		Description: p.Description, Builtin: true,
	}
}

func storedPresetDTO(p store.Preset) presetDTO {
	return presetDTO{
		ID: p.ID, Role: p.Role, Level: p.Level, InterviewType: p.InterviewType,
		Difficulty: p.Difficulty, QuestionCount: p.QuestionCount,
		FocusAreas: strSliceOrEmpty(p.FocusAreas), JDSample: p.JDSample,
		Description: p.Description, Builtin: false, CreatedBy: p.CreatedBy,
	}
}

func strSliceOrEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// handleInterviewPresets is readable by every authenticated user: built-in
// presets first (stable order), then the admin's (newest first).
func (s *Server) handleInterviewPresets(w http.ResponseWriter, r *http.Request) {
	stored, err := s.store.ListPresets(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	builtins := interview.Presets()
	out := make([]presetDTO, 0, len(builtins)+len(stored))
	for _, p := range builtins {
		out = append(out, builtinPresetDTO(p))
	}
	for _, p := range stored {
		out = append(out, storedPresetDTO(p))
	}
	writeOK(w, map[string]any{
		"presets": out,
		"labels":  interview.AllLabels(),
	})
}

// presetRequest is the create/update body. Pointers so an update can tell
// "absent" (leave alone) from "" (clear).
type presetRequest struct {
	Role          *string   `json:"role"`
	Level         *string   `json:"level"`
	InterviewType *string   `json:"interviewType"`
	Difficulty    *string   `json:"difficulty"`
	QuestionCount *int      `json:"questionCount"`
	FocusAreas    *[]string `json:"focusAreas"`
	JDSample      *string   `json:"jdSample"`
	Description   *string   `json:"description"`
}

// normalizePresetInput validates and repairs a preset body. Unknown enum
// values fall back to the same defaults interview creation uses, so a typo
// in a form field cannot make the preset unusable.
func normalizePresetInput(role, level, typ, difficulty string, count int) (interview.Spec, error) {
	if strings.TrimSpace(role) == "" {
		return interview.Spec{}, errors.New("role is required")
	}
	spec := interview.Spec{
		Role: role, Level: level, Type: typ, Difficulty: difficulty,
		QuestionCount: count,
	}.Normalize()
	return spec, nil
}

func (s *Server) handleCreatePreset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	ident, _ := auth.FromContext(r.Context())
	var req presetRequest
	if !readJSON(w, r, &req) {
		return
	}
	spec, err := normalizePresetInput(
		derefStr(req.Role), derefStr(req.Level), derefStr(req.InterviewType),
		derefStr(req.Difficulty), derefInt(req.QuestionCount, 6))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := randomID("ps_")
	if err != nil {
		storeError(w, err)
		return
	}
	p := &store.Preset{
		ID: id, Role: spec.Role, Level: spec.Level, InterviewType: spec.Type,
		Difficulty: spec.Difficulty, QuestionCount: spec.QuestionCount,
		FocusAreas: derefStrSlice(req.FocusAreas),
		JDSample:   derefStr(req.JDSample), Description: derefStr(req.Description),
		CreatedBy: ident.UserID,
	}
	if err := s.store.CreatePreset(r.Context(), p); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "preset": storedPresetDTO(*p)})
}

// loadEditablePreset fetches a database preset and refuses built-ins and
// rows that do not exist.
func (s *Server) loadEditablePreset(w http.ResponseWriter, r *http.Request) (*store.Preset, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if interview.IsBuiltinPreset(id) {
		writeError(w, http.StatusConflict, "内置预置不可修改")
		return nil, false
	}
	p, err := s.store.GetPreset(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return nil, false
	}
	return p, true
}

func (s *Server) handleUpdatePreset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	p, ok := s.loadEditablePreset(w, r)
	if !ok {
		return
	}
	var req presetRequest
	if !readJSON(w, r, &req) {
		return
	}
	role := p.Role
	if req.Role != nil {
		role = *req.Role
	}
	level, typ, difficulty := p.Level, p.InterviewType, p.Difficulty
	if req.Level != nil {
		level = *req.Level
	}
	if req.InterviewType != nil {
		typ = *req.InterviewType
	}
	if req.Difficulty != nil {
		difficulty = *req.Difficulty
	}
	count := p.QuestionCount
	if req.QuestionCount != nil {
		count = *req.QuestionCount
	}
	spec, err := normalizePresetInput(role, level, typ, difficulty, count)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p.Role, p.Level, p.InterviewType = spec.Role, spec.Level, spec.Type
	p.Difficulty, p.QuestionCount = spec.Difficulty, spec.QuestionCount
	if req.FocusAreas != nil {
		p.FocusAreas = *req.FocusAreas
	}
	if req.JDSample != nil {
		p.JDSample = *req.JDSample
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if err := s.store.UpdatePreset(r.Context(), p); err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"preset": storedPresetDTO(*p)})
}

func (s *Server) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if interview.IsBuiltinPreset(id) {
		writeError(w, http.StatusConflict, "内置预置不可删除")
		return
	}
	if err := s.store.DeletePreset(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, nil)
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func derefInt(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

func derefStrSlice(p *[]string) []string {
	if p == nil {
		return []string{}
	}
	out := make([]string, 0, len(*p))
	for _, s := range *p {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
