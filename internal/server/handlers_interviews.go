package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/events"
	"github.com/Potterluo/dream-interviewer/internal/interview"
	"github.com/Potterluo/dream-interviewer/internal/store"
)

// handlers_interviews.go: the product. One CRUD resource plus the single
// streaming endpoint that drives an interview forward.
//
// Everything about a turn — grading it, picking the next question,
// deciding to follow up, and knowing when to stop — lives behind
// POST /api/interviews/{id}/advance. One endpoint means one client code
// path and one place where the state machine can be reasoned about; the
// alternative (separate endpoints for ask/answer/grade/report) would let
// the client drive the interview into states the server never intended.
//
// The SSE frame vocabulary is documented in docs/API.md §3.

const (
	maxFollowUpsPerInterview = 3
	maxFollowUpsPerQuestion  = 1
)

// --- DTOs -------------------------------------------------------------------

type interviewDTO struct {
	ID             string            `json:"id"`
	UserID         string            `json:"userId"`
	Title          string            `json:"title"`
	Role           string            `json:"role"`
	Level          string            `json:"level"`
	InterviewType  string            `json:"interviewType"`
	Language       string            `json:"language"`
	Difficulty     string            `json:"difficulty"`
	QuestionCount  int               `json:"questionCount"`
	ResumeText     string            `json:"resumeText"`
	JDText         string            `json:"jdText"`
	Status         string            `json:"status"`
	Plan           *interview.Plan   `json:"plan"`
	CurrentSeq     int               `json:"currentSeq"`
	TurnCount      int               `json:"turnCount"`
	OverallScore   float64           `json:"overallScore"`
	HRSatisfaction float64           `json:"hrSatisfaction"`
	Recommendation string            `json:"recommendation"`
	Summary        string            `json:"summary"`
	Report         *interview.Report `json:"report"`
	Model          string            `json:"model"`
	Engine         string            `json:"engine"`
	DurationSec    int               `json:"durationSec"`
	StartedAt      *time.Time        `json:"startedAt"`
	CompletedAt    *time.Time        `json:"completedAt"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

type interviewTurnDTO struct {
	ID            string           `json:"id"`
	InterviewID   string           `json:"interviewId"`
	Seq           int              `json:"seq"`
	Kind          string           `json:"kind"`
	Dimension     string           `json:"dimension"`
	Question      string           `json:"question"`
	Intent        string           `json:"intent"`
	Weight        float64          `json:"weight"`
	Answer        string           `json:"answer"`
	Score         float64          `json:"score"`
	Grade         *interview.Grade `json:"grade"`
	Feedback      string           `json:"feedback"`
	AnswerSeconds int              `json:"answerSeconds"`
	CreatedAt     time.Time        `json:"createdAt"`
	AnsweredAt    *time.Time       `json:"answeredAt"`
}

// toInterviewDTO maps a row to its wire shape. full=false is the list
// projection: the heavy model artifacts and the candidate's own résumé/JD
// text are dropped so a table of 50 interviews is not megabytes.
func toInterviewDTO(iv *store.Interview, full bool) interviewDTO {
	dto := interviewDTO{
		ID: iv.ID, UserID: iv.UserID, Title: iv.Title, Role: iv.Role,
		Level: iv.Level, InterviewType: iv.InterviewType, Language: iv.Language,
		Difficulty: iv.Difficulty, QuestionCount: iv.QuestionCount,
		Status: iv.Status, CurrentSeq: iv.CurrentSeq, TurnCount: iv.TurnCount,
		OverallScore: iv.OverallScore, HRSatisfaction: iv.HRSatisfaction,
		Recommendation: iv.Recommendation, Model: iv.Model, Engine: iv.Engine,
		DurationSec: iv.DurationSec, StartedAt: iv.StartedAt,
		CompletedAt: iv.CompletedAt, CreatedAt: iv.CreatedAt, UpdatedAt: iv.UpdatedAt,
	}
	if full {
		dto.ResumeText = iv.ResumeText
		dto.JDText = iv.JDText
		dto.Summary = iv.Summary
		dto.Plan = parsePlan(iv.PlanJSON)
		dto.Report = parseReport(iv.ReportJSON)
	}
	return dto
}

func toTurnDTO(t *store.InterviewTurn) interviewTurnDTO {
	return interviewTurnDTO{
		ID: t.ID, InterviewID: t.InterviewID, Seq: t.Seq, Kind: t.Kind,
		Dimension: t.Dimension, Question: t.Question, Intent: t.Intent,
		Weight: t.Weight, Answer: t.Answer, Score: t.Score,
		Grade: parseGrade(t.GradeJSON), Feedback: t.Feedback,
		AnswerSeconds: t.AnswerSeconds, CreatedAt: t.CreatedAt,
		AnsweredAt: t.AnsweredAt,
	}
}

func toTurnDTOs(turns []store.InterviewTurn) []interviewTurnDTO {
	out := make([]interviewTurnDTO, 0, len(turns))
	for i := range turns {
		out = append(out, toTurnDTO(&turns[i]))
	}
	return out
}

// --- tolerant JSON column decoding ------------------------------------------

// A malformed JSON column must not take down a list page: the row is
// still useful (its real columns are intact), so these return nil and the
// UI treats the artifact as absent.

func parsePlan(raw string) *interview.Plan {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var p interview.Plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil
	}
	return &p
}

func parseReport(raw string) *interview.Report {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var r interview.Report
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil
	}
	return &r
}

func parseGrade(raw string) *interview.Grade {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var g interview.Grade
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return nil
	}
	return &g
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// --- shared helpers ---------------------------------------------------------

func specOf(iv *store.Interview) interview.Spec {
	return interview.Spec{
		Role:          iv.Role,
		Level:         iv.Level,
		Type:          iv.InterviewType,
		Language:      iv.Language,
		Difficulty:    iv.Difficulty,
		QuestionCount: iv.QuestionCount,
		Resume:        iv.ResumeText,
		JD:            iv.JDText,
	}
}

// briefsOf projects turns for the engine's history and report input.
//
// It keeps only turns that were actually ANSWERED (answered_at set). That
// matters twice over: the question currently on screen is passed to the
// grader separately as its own field, so leaving it in the transcript would
// show the model the very question it is grading marked "（未作答）"; and a
// report built from an abandoned session would otherwise tell the candidate
// they left a question unanswered when they simply quit. A SKIPPED turn is
// answered (with no text), so it still counts as 未作答 in the report — which
// is the distinction we actually want.
func briefsOf(turns []store.InterviewTurn) []interview.TurnBrief {
	out := make([]interview.TurnBrief, 0, len(turns))
	for i := range turns {
		t := &turns[i]
		if t.AnsweredAt == nil {
			continue
		}
		brief := interview.TurnBrief{
			Seq: t.Seq, Kind: t.Kind, Dimension: t.Dimension,
			Question: t.Question, Answer: t.Answer, Score: t.Score,
			Feedback: t.Feedback,
		}
		if g := parseGrade(t.GradeJSON); g != nil {
			brief.DimScores = g.Dimensions
		}
		out = append(out, brief)
	}
	return out
}

// loadOwnedInterview fetches the row and enforces ownership: the owner
// and admins pass, everyone else gets the same 404 as a missing row.
func (s *Server) loadOwnedInterview(w http.ResponseWriter, r *http.Request) (*store.Interview, bool) {
	ident, _ := auth.FromContext(r.Context())
	iv, err := s.store.GetInterview(r.Context(), r.PathValue("id"))
	if err != nil {
		storeError(w, err)
		return nil, false
	}
	if iv.UserID != ident.UserID && !ident.IsAdmin() {
		writeError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return iv, true
}

// --- CRUD -------------------------------------------------------------------

func (s *Server) handleListInterviews(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveScope(w, r)
	if !ok {
		return
	}
	rows, err := s.store.ListInterviews(r.Context(), userID)
	if err != nil {
		storeError(w, err)
		return
	}
	out := make([]interviewDTO, 0, len(rows))
	for i := range rows {
		out = append(out, toInterviewDTO(&rows[i], false))
	}
	writeOK(w, map[string]any{"interviews": out})
}

// createInterviewRequest is shared by create and update.
type createInterviewRequest struct {
	Title         *string `json:"title"`
	Role          *string `json:"role"`
	Level         *string `json:"level"`
	InterviewType *string `json:"interviewType"`
	Language      *string `json:"language"`
	Difficulty    *string `json:"difficulty"`
	QuestionCount *int    `json:"questionCount"`
	ResumeText    *string `json:"resumeText"`
	JDText        *string `json:"jdText"`
}

func (s *Server) handleCreateInterview(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	var req createInterviewRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.Role == nil || strings.TrimSpace(*req.Role) == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}

	spec := interview.Spec{
		Role: str(req.Role), Level: str(req.Level), Type: str(req.InterviewType),
		Language: str(req.Language), Difficulty: str(req.Difficulty),
		QuestionCount: intOr(req.QuestionCount, 6),
		Resume:        str(req.ResumeText), JD: str(req.JDText),
	}.Normalize()

	id, err := randomID("iv_")
	if err != nil {
		storeError(w, err)
		return
	}
	iv := &store.Interview{
		ID: id, UserID: ident.EffectiveUserID(),
		Title:         defaultTitle(req.Title, spec),
		Role:          spec.Role,
		Level:         spec.Level,
		InterviewType: spec.Type,
		Language:      spec.Language,
		Difficulty:    spec.Difficulty,
		QuestionCount: spec.QuestionCount,
		ResumeText:    spec.Resume,
		JDText:        spec.JD,
		Status:        interview.StatusDraft,
	}
	if err := s.store.CreateInterview(r.Context(), iv); err != nil {
		storeError(w, err)
		return
	}
	s.publishInterview("interview.created", iv)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "interview": toInterviewDTO(iv, true)})
}

func (s *Server) handleGetInterview(w http.ResponseWriter, r *http.Request) {
	iv, ok := s.loadOwnedInterview(w, r)
	if !ok {
		return
	}
	turns, err := s.store.ListInterviewTurns(r.Context(), iv.ID)
	if err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{
		"interview": toInterviewDTO(iv, true),
		"turns":     toTurnDTOs(turns),
	})
}

func (s *Server) handleUpdateInterview(w http.ResponseWriter, r *http.Request) {
	iv, ok := s.loadOwnedInterview(w, r)
	if !ok {
		return
	}
	if iv.Status != interview.StatusDraft {
		writeError(w, http.StatusConflict, "面试已开始，无法再修改配置")
		return
	}
	var req createInterviewRequest
	if !readJSON(w, r, &req) {
		return
	}
	spec := interview.Spec{
		Role:          firstNonEmptyStr(str(req.Role), iv.Role),
		Level:         firstNonEmptyStr(str(req.Level), iv.Level),
		Type:          firstNonEmptyStr(str(req.InterviewType), iv.InterviewType),
		Language:      firstNonEmptyStr(str(req.Language), iv.Language),
		Difficulty:    firstNonEmptyStr(str(req.Difficulty), iv.Difficulty),
		QuestionCount: intOr(req.QuestionCount, iv.QuestionCount),
		Resume:        firstNonEmptyStr(str(req.ResumeText), iv.ResumeText),
		JD:            firstNonEmptyStr(str(req.JDText), iv.JDText),
	}.Normalize()

	iv.Role = spec.Role
	iv.Level = spec.Level
	iv.InterviewType = spec.Type
	iv.Language = spec.Language
	iv.Difficulty = spec.Difficulty
	iv.QuestionCount = spec.QuestionCount
	iv.ResumeText = spec.Resume
	iv.JDText = spec.JD
	if req.Title != nil {
		iv.Title = strings.TrimSpace(*req.Title)
	}
	if iv.Title == "" {
		iv.Title = defaultTitle(nil, spec)
	}

	if err := s.store.UpdateInterview(r.Context(), iv); err != nil {
		storeError(w, err)
		return
	}
	s.publishInterview("interview.updated", iv)
	writeOK(w, map[string]any{"interview": toInterviewDTO(iv, true)})
}

func (s *Server) handleDeleteInterview(w http.ResponseWriter, r *http.Request) {
	iv, ok := s.loadOwnedInterview(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteInterview(r.Context(), iv.ID); err != nil {
		storeError(w, err)
		return
	}
	s.hub.PublishTo(iv.UserID, events.Event{
		Type: "interview.deleted", Data: map[string]string{"id": iv.ID},
	})
	writeOK(w, nil)
}

func (s *Server) publishInterview(eventType string, iv *store.Interview) {
	s.hub.PublishTo(iv.UserID, events.Event{Type: eventType, Data: toInterviewDTO(iv, false)})
}

// --- the state machine ------------------------------------------------------

// advanceRequest is the body of POST /api/interviews/{id}/advance.
type advanceRequest struct {
	Action     string `json:"action"`
	Answer     string `json:"answer"`
	ElapsedSec int    `json:"elapsedSec"`
	// TurnID optionally binds this submit to the question the client was
	// looking at. Without it, a retry that arrives after the interview has
	// moved on would be written against whatever question is open NOW —
	// see checkTurnBinding.
	TurnID string `json:"turnId"`
}

// sseWriter wraps the frame protocol so every emit site looks the same
// and a closed connection is reported the same way everywhere.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// frame writes one `data: {"type":…,"data":…}` event. It returns false
// once the client is gone, which every caller treats as "stop working".
func (sw *sseWriter) frame(frameType string, data any) bool {
	payload, err := json.Marshal(map[string]any{"type": frameType, "data": data})
	if err != nil {
		return true // unserialisable frame: skip it, keep the stream
	}
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", payload); err != nil {
		return false
	}
	sw.flusher.Flush()
	return true
}

func (sw *sseWriter) stage(name string) bool { return sw.frame("stage", name) }

func (sw *sseWriter) errorFrame(msg string) { sw.frame("error", msg) }

// noteFallback tells the candidate — in the same stream they are already
// reading — that this half came from the offline rubric, and why.
// Silently presenting heuristic output as a model's judgement would be
// the most misleading thing this product could do, so the reason travels
// with the output instead of only reaching a server log.
func (sw *sseWriter) noteFallback(source interview.Source) {
	if source.Engine != interview.EngineOffline || source.Note == "" {
		return
	}
	sw.frame("delta", fmt.Sprintf(
		"\n\n> ⚠️ **已切换为内置题库与评分规则**，本次结果不是模型生成的。原因：%s\n", source.Note))
}

func (s *Server) handleAdvanceInterview(w http.ResponseWriter, r *http.Request) {
	iv, ok := s.loadOwnedInterview(w, r)
	if !ok {
		return
	}
	var req advanceRequest
	if !readJSON(w, r, &req) {
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))

	// Validate the transition BEFORE opening the stream, so a bad request
	// is a real HTTP error the client can branch on rather than an error
	// frame buried inside a 200.
	if err := checkTransition(iv, action); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if action == "answer" && strings.TrimSpace(req.Answer) == "" {
		writeError(w, http.StatusBadRequest, "answer must not be empty (use action=skip)")
		return
	}

	// One advance at a time per interview, and a CONCURRENT duplicate is
	// rejected rather than queued.
	//
	// Queuing would be a correctness bug, not just a cost one: by the time
	// the loser acquired the lock the winner would have advanced the
	// interview, so the loser would find a different open question and
	// happily write the stale submission onto it. The compare-and-set
	// cannot catch that (different row), which is why the duplicate has to
	// be refused up front. Verified against the previous behaviour.
	unlock, acquired := s.advanceLocks.TryLock(iv.ID)
	if !acquired {
		writeError(w, http.StatusConflict,
			"这场面试正在处理上一个请求，请等它结束后重试")
		return
	}
	defer unlock()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sw := &sseWriter{w: w, flusher: flusher}

	// visible forwards a model delta to the browser. The bool return is
	// what stops generation the moment the candidate disconnects.
	visible := func(text string) error {
		if !sw.frame("delta", text) {
			return errClientGone
		}
		return nil
	}

	switch action {
	case "start":
		s.advanceStart(r.Context(), sw, iv, visible)
	case "answer":
		s.advanceAnswer(r.Context(), sw, iv, req, visible)
	case "skip":
		s.advanceSkip(r.Context(), sw, iv, req)
	case "finish":
		s.advanceFinish(r.Context(), sw, iv, visible)
	case "abort":
		s.advanceAbort(r.Context(), sw, iv)
	}
}

// errClientGone is returned by the delta callback to abort a model call
// as soon as the browser goes away.
var errClientGone = fmt.Errorf("client disconnected")

func checkTransition(iv *store.Interview, action string) error {
	switch action {
	case "start":
		if iv.Status == interview.StatusCompleted {
			return fmt.Errorf("面试已结束，如需重新评估请使用重新生成报告")
		}
	case "answer", "skip":
		if iv.Status != interview.StatusInProgress {
			return fmt.Errorf("面试尚未开始或已结束")
		}
	case "finish":
		// `aborted` is included on purpose: giving up on ANSWERING is not
		// the same as not wanting the evaluation, and the interview room
		// offers "生成报告" on an abandoned session. It moves the row to
		// `completed` like any other wrap-up.
		if iv.Status != interview.StatusInProgress &&
			iv.Status != interview.StatusCompleted &&
			iv.Status != interview.StatusAborted {
			return fmt.Errorf("面试尚未开始，无法生成报告")
		}
	case "abort":
		if iv.Status == interview.StatusCompleted {
			return fmt.Errorf("面试已结束并生成报告，无需放弃")
		}
		if iv.Status == interview.StatusDraft {
			// Nothing has been asked, so there is nothing to abandon — and
			// allowing it would leave a row with no plan, whose only
			// remaining action ("generate report") cannot work.
			return fmt.Errorf("面试还没有开始，无需放弃；直接删除即可")
		}
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}

// advanceAbort marks a session as given up.
//
// It exists so `aborted` is a state something actually writes: it is in
// the enum, the label table, the UI and the analytics counter, and
// without a path that sets it that counter could only ever read zero.
// Nothing is graded here — the candidate quit, and pretending to evaluate
// a half-finished interview would be inventing data.
func (s *Server) advanceAbort(ctx context.Context, sw *sseWriter, iv *store.Interview) {
	now := time.Now().UTC()
	iv.Status = interview.StatusAborted
	if iv.StartedAt != nil {
		iv.DurationSec = int(now.Sub(*iv.StartedAt).Seconds())
		if iv.DurationSec < 0 {
			iv.DurationSec = 0
		}
	}
	if err := s.store.UpdateInterview(ctx, iv); err != nil {
		sw.errorFrame("保存状态失败")
		return
	}
	s.publishInterview("interview.updated", iv)
	s.finishStream(ctx, sw, iv)
}

// advanceStart builds the plan (once) and asks the first question.
func (s *Server) advanceStart(ctx context.Context, sw *sseWriter, iv *store.Interview, visible func(string) error) {
	spec := specOf(iv)
	plan := parsePlan(iv.PlanJSON)

	if plan == nil || len(plan.Questions) == 0 {
		if !sw.stage("planning") {
			return
		}
		built, source, err := s.engine.get().BuildPlan(ctx, spec, visible)
		if err != nil {
			sw.errorFrame(friendlyEngineError(err))
			return
		}
		plan = &built
		iv.PlanJSON = mustJSON(built)
		iv.Model = source.Model
		iv.Engine = source.Engine
		sw.noteFallback(source)
		if !sw.frame("plan", built) {
			return
		}
	}

	turns, err := s.store.ListInterviewTurns(ctx, iv.ID)
	if err != nil {
		sw.errorFrame("读取答题记录失败")
		return
	}

	if pending := pendingTurn(turns); pending != nil {
		// Already has an open question: re-emit it instead of asking a new
		// one, so a page reload or a retried start cannot skip a question.
		// Refresh the counters from the rows we just read — the `done`
		// frame is documented as authoritative, so it must not disagree
		// with itself.
		iv.TurnCount = len(turns)
		iv.CurrentSeq = pending.Seq
		// `start` doubles as "resume": if the session was abandoned, put it
		// back in progress and persist, or the question we are about to
		// stream could not be answered (answer/skip require in_progress)
		// and the row would keep claiming to be aborted.
		if iv.Status == interview.StatusAborted {
			iv.Status = interview.StatusInProgress
			s.persistInterview(ctx, iv)
		}
		sw.stage("questioning")
		_ = sw.frame("delta", pending.Question)
		sw.frame("question", map[string]any{"turn": toTurnDTO(pending)})
		s.finishStream(ctx, sw, iv)
		return
	}

	if len(turns) > 0 {
		// History exists but nothing is open: the previous request died
		// between writing the grade and asking the next question. Resume
		// the plan — never delete the answers here. (An earlier version
		// wiped every turn in this branch, which would have thrown away a
		// candidate's whole session if a retried start landed in that
		// window.)
		askedPlan := countKind(turns, interview.KindQuestion)
		if askedPlan >= len(plan.Questions) {
			// Nothing left to ask; the report is the only remaining step.
			s.advanceFinishWith(ctx, sw, iv, plan, turns,
				interview.Source{Engine: iv.Engine, Model: iv.Model}, visible)
			return
		}
		iv.Status = interview.StatusInProgress
		iv.TurnCount = len(turns) + 1
		if err := s.askQuestion(ctx, sw, iv, plan.Questions[askedPlan], interview.KindQuestion, turns[len(turns)-1].Seq+1); err != nil {
			sw.errorFrame(err.Error())
			return
		}
		s.persistInterview(ctx, iv)
		s.finishStream(ctx, sw, iv)
		return
	}

	now := time.Now().UTC()
	if iv.StartedAt == nil {
		iv.StartedAt = &now
	}
	iv.Status = interview.StatusInProgress
	// +1 for the question askQuestion is about to create: `turns` is the
	// list as it was BEFORE that insert, and writing len(turns) here would
	// persist a count that is off by one for the whole session.
	iv.TurnCount = len(turns) + 1

	next := plan.Questions[0]
	if err := s.askQuestion(ctx, sw, iv, next, interview.KindQuestion, 1); err != nil {
		sw.errorFrame(err.Error())
		return
	}
	s.persistInterview(ctx, iv)
	s.finishStream(ctx, sw, iv)
}

// advanceAnswer grades the open question, then asks the next one or
// wraps the interview up.
func (s *Server) advanceAnswer(ctx context.Context, sw *sseWriter, iv *store.Interview, req advanceRequest, visible func(string) error) {
	turns, err := s.store.ListInterviewTurns(ctx, iv.ID)
	if err != nil {
		sw.errorFrame("读取答题记录失败")
		return
	}
	pending := pendingTurn(turns)
	if pending == nil {
		sw.errorFrame("当前没有待作答的题目")
		return
	}
	if err := checkTurnBinding(req.TurnID, pending); err != nil {
		sw.errorFrame(err.Error())
		return
	}
	plan := parsePlan(iv.PlanJSON)
	if plan == nil || len(plan.Questions) == 0 {
		sw.errorFrame("面试大纲已丢失，请重新开始面试")
		return
	}
	spec := specOf(iv)

	question := plannedFor(plan, pending)
	followUpsUsed := countKind(turns, interview.KindFollowup)
	briefs := briefsOf(turns)

	if !sw.stage("grading") {
		return
	}
	grade, source, err := s.engine.get().GradeAnswer(ctx, interview.GradeInput{
		Spec: spec, Plan: *plan, Question: question, Kind: pending.Kind,
		Answer: req.Answer, History: briefs, Answered: len(turns),
		FollowUps: followUpsUsed,
	}, visible)
	if err != nil {
		sw.errorFrame(friendlyEngineError(err))
		return
	}

	now := time.Now().UTC()
	pending.Answer = req.Answer
	pending.Score = grade.Score
	pending.GradeJSON = mustJSON(grade)
	pending.Feedback = grade.Feedback
	pending.AnsweredAt = &now
	pending.AnswerSeconds = req.ElapsedSec
	if err := s.store.AnswerInterviewTurn(ctx, pending); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Another request (double click, retry, second tab) got there
			// first. Say so plainly instead of overwriting its grade.
			sw.errorFrame("这道题已经作答过了，请刷新页面查看最新的面试状态")
			return
		}
		sw.errorFrame("保存评分失败")
		return
	}
	// Reflect the written fields back into the local slice so the frames
	// below and the terminal `done` frame agree.
	for i := range turns {
		if turns[i].ID == pending.ID {
			turns[i] = *pending
		}
	}
	// A transient source (a contentless answer scored by the rubric) is
	// reported to the candidate but must not relabel the interview.
	if !source.Transient {
		iv.Model = source.Model
		iv.Engine = source.Engine
	}

	sw.noteFallback(source)
	if !sw.frame("grade", map[string]any{"turn": toTurnDTO(pending)}) {
		return
	}
	s.continueInterview(ctx, sw, iv, plan, turns, grade, source, visible)
}

// advanceSkip records an unanswered question at 0 and moves on.
func (s *Server) advanceSkip(ctx context.Context, sw *sseWriter, iv *store.Interview, req advanceRequest) {
	turns, err := s.store.ListInterviewTurns(ctx, iv.ID)
	if err != nil {
		sw.errorFrame("读取答题记录失败")
		return
	}
	pending := pendingTurn(turns)
	if pending == nil {
		sw.errorFrame("当前没有待作答的题目")
		return
	}
	if err := checkTurnBinding(req.TurnID, pending); err != nil {
		sw.errorFrame(err.Error())
		return
	}
	plan := parsePlan(iv.PlanJSON)
	if plan == nil || len(plan.Questions) == 0 {
		sw.errorFrame("面试大纲已丢失，请重新开始面试")
		return
	}

	zero := map[string]float64{}
	if dims := plan.Dimensions; len(dims) > 0 {
		for _, d := range dims {
			zero[d.Key] = 0
		}
	}
	grade := interview.Grade{
		Score:      0,
		Dimensions: zero,
		Strengths:  []string{},
		Weaknesses: []string{"本题未作答"},
		Feedback: "（本题已跳过，按 0 分计入总评。" +
			"遇到不会的题目时，先给出解题思路和你能确定的部分，通常能拿到框架分。）",
		Reference: plannedFor(plan, pending).Reference,
		Verdict:   interview.VerdictWeak,
	}
	now := time.Now().UTC()
	pending.Score = 0
	pending.GradeJSON = mustJSON(grade)
	pending.Feedback = grade.Feedback
	pending.AnsweredAt = &now
	if err := s.store.AnswerInterviewTurn(ctx, pending); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			sw.errorFrame("这道题已经作答或跳过了，请刷新页面查看最新的面试状态")
			return
		}
		sw.errorFrame("保存记录失败")
		return
	}
	for i := range turns {
		if turns[i].ID == pending.ID {
			turns[i] = *pending
		}
	}
	if !sw.frame("grade", map[string]any{"turn": toTurnDTO(pending)}) {
		return
	}
	// A skipped question is never followed up — there is nothing to dig
	// into, and pressing costs the candidate time for no signal.
	s.continueInterview(ctx, sw, iv, plan, turns, grade, interview.Source{
		Engine: iv.Engine, Model: iv.Model,
	}, nil)
}

// continueInterview decides what comes after a graded (or skipped)
// answer: an adaptive follow-up, the next planned question, or the end.
func (s *Server) continueInterview(
	ctx context.Context, sw *sseWriter, iv *store.Interview,
	plan *interview.Plan, turns []store.InterviewTurn,
	grade interview.Grade, source interview.Source, visible func(string) error,
) {
	askedPlan := countKind(turns, interview.KindQuestion)
	followUpsUsed := countKind(turns, interview.KindFollowup)
	answered := turns[len(turns)-1]

	// A follow-up is only worth it while the plan still has room left;
	// following up on the final question would mean answering forever.
	wantFollowUp := grade.FollowUp != "" &&
		followUpsUsed < maxFollowUpsPerInterview &&
		answered.Kind != interview.KindFollowup &&
		askedPlan < len(plan.Questions)

	total := len(turns) + 1
	if wantFollowUp && total <= len(plan.Questions)+maxFollowUpsPerInterview {
		// The follow-up inherits the parent question's keywords so the
		// relevance signal keeps working on the deeper question.
		question := interview.PlannedQuestion{
			Seq:       answered.Seq + 1,
			Dimension: answered.Dimension,
			Question:  grade.FollowUp,
			Intent:    "追问：" + answered.Intent,
			Weight:    1,
			Reference: grade.Reference,
			Keywords:  plannedFor(plan, &answered).Keywords,
		}
		if err := s.askQuestion(ctx, sw, iv, question, interview.KindFollowup, answered.Seq+1); err != nil {
			sw.errorFrame(err.Error())
			return
		}
		iv.TurnCount = total
		s.persistInterview(ctx, iv)
		s.finishStream(ctx, sw, iv)
		return
	}

	if askedPlan < len(plan.Questions) {
		next := plan.Questions[askedPlan]
		if err := s.askQuestion(ctx, sw, iv, next, interview.KindQuestion, answered.Seq+1); err != nil {
			sw.errorFrame(err.Error())
			return
		}
		iv.TurnCount = total
		s.persistInterview(ctx, iv)
		s.finishStream(ctx, sw, iv)
		return
	}

	// Plan exhausted — write the report and close the interview.
	s.advanceFinishWith(ctx, sw, iv, plan, turns, source, visible)
}

// advanceFinish is the explicit "wrap up now" action.
func (s *Server) advanceFinish(ctx context.Context, sw *sseWriter, iv *store.Interview, visible func(string) error) {
	turns, err := s.store.ListInterviewTurns(ctx, iv.ID)
	if err != nil {
		sw.errorFrame("读取答题记录失败")
		return
	}
	plan := parsePlan(iv.PlanJSON)
	if plan == nil {
		sw.errorFrame("面试大纲已丢失，无法生成报告")
		return
	}
	s.advanceFinishWith(ctx, sw, iv, plan, turns, interview.Source{
		Engine: iv.Engine, Model: iv.Model,
	}, visible)
}

// advanceFinishWith writes the report, marks the interview completed and
// closes the stream.
func (s *Server) advanceFinishWith(
	ctx context.Context, sw *sseWriter, iv *store.Interview,
	plan *interview.Plan, turns []store.InterviewTurn,
	source interview.Source, visible func(string) error,
) {
	if !sw.stage("reporting") {
		return
	}
	report, reportSource, err := s.engine.get().BuildReport(ctx, interview.ReportInput{
		Spec: specOf(iv), Plan: *plan, Turns: briefsOf(turns),
	}, visible)
	if err != nil {
		sw.errorFrame(friendlyEngineError(err))
		return
	}

	now := time.Now().UTC()
	iv.Status = interview.StatusCompleted
	iv.CompletedAt = &now
	iv.ReportJSON = mustJSON(report)
	iv.Summary = report.Summary
	iv.OverallScore = report.OverallScore
	iv.HRSatisfaction = report.HRSatisfaction
	iv.Recommendation = report.Recommendation
	iv.TurnCount = len(turns)
	if reportSource.Model != "" {
		iv.Model = reportSource.Model
	}
	iv.Engine = reportSource.Engine
	if iv.StartedAt != nil {
		iv.DurationSec = int(now.Sub(*iv.StartedAt).Seconds())
		if iv.DurationSec < 0 {
			iv.DurationSec = 0
		}
	}
	if err := s.store.UpdateInterview(ctx, iv); err != nil {
		sw.errorFrame("保存报告失败")
		return
	}
	sw.noteFallback(reportSource)
	if !sw.frame("report", report) {
		return
	}
	s.publishInterview("interview.updated", iv)
	s.finishStream(ctx, sw, iv)
}

// askQuestion mints the next turn row and emits it as a streamed
// question: a `stage` marker, the question text as deltas (so it types
// out), then the structured `question` frame.
func (s *Server) askQuestion(
	ctx context.Context, sw *sseWriter, iv *store.Interview,
	q interview.PlannedQuestion, kind string, seq int,
) error {
	id, err := randomID("tr_")
	if err != nil {
		return fmt.Errorf("生成题目 ID 失败")
	}
	turn := &store.InterviewTurn{
		ID: id, InterviewID: iv.ID, UserID: iv.UserID, Seq: seq, Kind: kind,
		Dimension: q.Dimension, Question: q.Question, Intent: q.Intent,
		Weight: q.Weight,
	}
	if turn.Weight <= 0 {
		turn.Weight = 1
	}
	if err := s.store.CreateInterviewTurn(ctx, turn); err != nil {
		return fmt.Errorf("保存题目失败")
	}
	if !sw.stage("questioning") {
		return errClientGone
	}
	if !sw.frame("delta", q.Question) {
		return errClientGone
	}
	iv.CurrentSeq = seq
	if !sw.frame("question", map[string]any{"turn": toTurnDTO(turn)}) {
		return errClientGone
	}
	return nil
}

// finishStream emits the terminal frame with authoritative state.
func (s *Server) finishStream(ctx context.Context, sw *sseWriter, iv *store.Interview) {
	turns, err := s.store.ListInterviewTurns(ctx, iv.ID)
	if err != nil {
		sw.errorFrame("读取答题记录失败")
		return
	}
	sw.frame("done", map[string]any{
		"interview": toInterviewDTO(iv, true),
		"turns":     toTurnDTOs(turns),
	})
}

// persistInterview saves non-report progress (status, model, counters).
func (s *Server) persistInterview(ctx context.Context, iv *store.Interview) {
	if err := s.store.UpdateInterview(ctx, iv); err != nil {
		// The stream is already open, so this cannot become an HTTP error.
		// Losing it is a real bug, so record it rather than failing mute.
		slog.Error("persist interview progress", "id", iv.ID, "err", err)
		return
	}
	s.publishInterview("interview.updated", iv)
}

// --- helpers ----------------------------------------------------------------

// pendingTurn is the open question: the newest turn with no AnsweredAt.
// Using AnsweredAt (not an empty Answer) is what lets a skipped question
// stay answered while carrying no text.
func pendingTurn(turns []store.InterviewTurn) *store.InterviewTurn {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].AnsweredAt == nil {
			return &turns[i]
		}
	}
	return nil
}

func countKind(turns []store.InterviewTurn, kind string) int {
	n := 0
	for i := range turns {
		if turns[i].Kind == kind {
			n++
		}
	}
	return n
}

// plannedFor finds the plan entry a turn came from. Follow-up turns have
// no plan entry of their own, so they inherit the entry for the planned
// question in the same dimension.
func plannedFor(plan *interview.Plan, t *store.InterviewTurn) interview.PlannedQuestion {
	if t == nil || plan == nil {
		return interview.PlannedQuestion{Seq: 1, Weight: 1, Question: "（题目已丢失）"}
	}
	if t.Kind == interview.KindQuestion {
		if idx := countKindBefore(plan, t); idx >= 0 {
			return plan.Questions[idx]
		}
	}
	for _, q := range plan.Questions {
		if q.Dimension == t.Dimension {
			return q
		}
	}
	return interview.PlannedQuestion{
		Seq: t.Seq, Dimension: t.Dimension, Question: t.Question,
		Intent: t.Intent, Weight: t.Weight,
	}
}

// countKindBefore maps a planned question turn back to its plan index by
// counting how many planned questions precede it.
func countKindBefore(plan *interview.Plan, t *store.InterviewTurn) int {
	// Turn seq is monotonic and planned questions keep their order, so the
	// index is the number of planned-question turns before this one — which
	// the caller reconstructs from seq by scanning the plan for a matching
	// question text.
	for i, q := range plan.Questions {
		if q.Question == t.Question {
			return i
		}
	}
	return -1
}

// checkTurnBinding rejects a submit aimed at a question that is no longer
// open.
//
// The concurrency lock only covers simultaneous requests. A LATE retry —
// a client resending after a timeout, a user pressing back and resubmitting
// — arrives when the interview has already moved on, and without this check
// it would be scored against the new question. The client knows which turn
// it was showing, so binding is cheap; an empty TurnID means an older or
// scripted client and is allowed through.
func checkTurnBinding(turnID string, pending *store.InterviewTurn) error {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" || turnID == pending.ID {
		return nil
	}
	return errors.New("题目已经翻页了（你看到的题目已被作答或替换），请刷新页面后重试")
}

// friendlyEngineError turns engine/transport failures into Chinese text
// the candidate can act on, and recognises the disconnect case so a Stop
// click is not reported as a model failure.
func friendlyEngineError(err error) string {
	switch {
	case err == nil:
		return "未知错误"
	case errors.Is(err, errClientGone), errors.Is(err, context.Canceled):
		return "已停止生成"
	case errors.Is(err, context.DeadlineExceeded):
		return "生成超时：模型服务响应过慢，请重试或换一个模型（设置 → 引擎）"
	default:
		return "生成失败：" + err.Error()
	}
}

func defaultTitle(explicit *string, spec interview.Spec) string {
	if explicit != nil && strings.TrimSpace(*explicit) != "" {
		return strings.TrimSpace(*explicit)
	}
	return fmt.Sprintf("%s · %s", spec.Role, interview.LabelOf("type", spec.Type))
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func intOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// --- stats ------------------------------------------------------------------

type dayCount struct {
	Date  string  `json:"date"`
	Count float64 `json:"count"`
}

type dimensionAvg struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type labelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type interviewStats struct {
	Total                   int            `json:"total"`
	Completed               int            `json:"completed"`
	InProgress              int            `json:"inProgress"`
	Aborted                 int            `json:"aborted"`
	AvgScore                float64        `json:"avgScore"`
	BestScore               float64        `json:"bestScore"`
	AvgDurationSec          int            `json:"avgDurationSec"`
	AvgTurns                float64        `json:"avgTurns"`
	ScoreByDay              []dayCount     `json:"scoreByDay"`
	AvgScoreByDay           []dayCount     `json:"avgScoreByDay"`
	DimensionAvg            []dimensionAvg `json:"dimensionAvg"`
	RoleBreakdown           []labelCount   `json:"roleBreakdown"`
	RecommendationBreakdown []labelCount   `json:"recommendationBreakdown"`
	LevelBreakdown          []labelCount   `json:"levelBreakdown"`
}

const statsWindowDays = 14

func (s *Server) handleInterviewStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveScope(w, r)
	if !ok {
		return
	}

	summaries, err := s.store.ListInterviewSummaries(r.Context(), userID)
	if err != nil {
		storeError(w, err)
		return
	}
	blobs, err := s.store.ListInterviewReportJSON(r.Context(), userID)
	if err != nil {
		storeError(w, err)
		return
	}

	stats := interviewStats{
		ScoreByDay:              []dayCount{},
		AvgScoreByDay:           []dayCount{},
		DimensionAvg:            []dimensionAvg{},
		RoleBreakdown:           []labelCount{},
		RecommendationBreakdown: []labelCount{},
		LevelBreakdown:          []labelCount{},
	}

	createdPerDay := map[string]int{}
	scoreSumPerDay := map[string]float64{}
	scoreCountPerDay := map[string]int{}
	roleCounts := map[string]int{}
	recCounts := map[string]int{}
	levelCounts := map[string]int{}
	totalScore, totalTurns, totalDuration := 0.0, 0, 0

	for i := range summaries {
		sm := &summaries[i]
		stats.Total++
		switch sm.Status {
		case interview.StatusCompleted:
			stats.Completed++
			totalScore += sm.OverallScore
			if sm.OverallScore > stats.BestScore {
				stats.BestScore = sm.OverallScore
			}
			totalDuration += sm.DurationSec
			if sm.Recommendation != "" {
				recCounts[interview.LabelOf("recommendation", sm.Recommendation)]++
			}
		case interview.StatusInProgress:
			stats.InProgress++
		case interview.StatusAborted:
			stats.Aborted++
		}
		totalTurns += sm.TurnCount
		day := sm.CreatedAt.UTC().Format("2006-01-02")
		createdPerDay[day]++
		if sm.Status == interview.StatusCompleted {
			scoreSumPerDay[day] += sm.OverallScore
			scoreCountPerDay[day]++
		}
		if sm.Role != "" {
			roleCounts[sm.Role]++
		}
		levelCounts[interview.LabelOf("level", sm.Level)]++
	}

	if stats.Completed > 0 {
		stats.AvgScore = roundTo1(totalScore / float64(stats.Completed))
		stats.AvgDurationSec = int(float64(totalDuration)/float64(stats.Completed) + 0.5)
	}
	if stats.Total > 0 {
		stats.AvgTurns = roundTo1(float64(totalTurns) / float64(stats.Total))
	}

	// Per-dimension averages come from the reports themselves (the only
	// place dimension scores exist), so they are averaged across every
	// report that carries them.
	dimSums := map[string]float64{}
	dimCounts := map[string]int{}
	dimLabels := map[string]string{}
	for _, blob := range blobs {
		rep := parseReport(blob)
		if rep == nil {
			continue
		}
		for _, d := range rep.Dimensions {
			dimSums[d.Key] += d.Score
			dimCounts[d.Key]++
			if d.Label != "" {
				dimLabels[d.Key] = d.Label
			}
		}
	}
	dimKeys := make([]string, 0, len(dimCounts))
	for k := range dimCounts {
		dimKeys = append(dimKeys, k)
	}
	sort.Strings(dimKeys)
	for _, k := range dimKeys {
		label := dimLabels[k]
		if label == "" {
			label = k
		}
		stats.DimensionAvg = append(stats.DimensionAvg, dimensionAvg{
			Key: k, Label: label,
			Score: roundTo1(dimSums[k] / float64(dimCounts[k])),
		})
	}

	for _, d := range lastNDays(statsWindowDays) {
		stats.ScoreByDay = append(stats.ScoreByDay, dayCount{Date: d, Count: float64(createdPerDay[d])})
		avg := 0.0
		if scoreCountPerDay[d] > 0 {
			avg = roundTo1(scoreSumPerDay[d] / float64(scoreCountPerDay[d]))
		}
		stats.AvgScoreByDay = append(stats.AvgScoreByDay, dayCount{Date: d, Count: avg})
	}

	stats.RoleBreakdown = topLabelCounts(roleCounts, 8)
	stats.RecommendationBreakdown = orderedLabelCounts(recCounts,
		[]string{interview.RecStrongHire, interview.RecHire, interview.RecMaybe, interview.RecNoHire},
		"recommendation")
	stats.LevelBreakdown = orderedLabelCounts(levelCounts,
		[]string{"初级", "中级", "高级", "资深/专家"}, "")

	writeOK(w, map[string]any{"stats": stats})
}

// topLabelCounts sorts a label→count map by count desc then label, and
// keeps the top n.
func topLabelCounts(m map[string]int, n int) []labelCount {
	out := make([]labelCount, 0, len(m))
	for k, v := range m {
		out = append(out, labelCount{Label: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// orderedLabelCounts emits a fixed order (so a legend never reshuffles),
// translating keys through the label set when one is given. Counts of 0
// are kept: a pie chart with a stable legend beats one that changes shape.
func orderedLabelCounts(m map[string]int, order []string, labelSet string) []labelCount {
	out := make([]labelCount, 0, len(order))
	for _, k := range order {
		label := k
		if labelSet != "" {
			label = interview.LabelOf(labelSet, k)
		}
		out = append(out, labelCount{Label: label, Count: m[label]})
	}
	return out
}

func roundTo1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// lastNDays returns the last n UTC dates, oldest first.
func lastNDays(n int) []string {
	out := make([]string, 0, n)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for i := n - 1; i >= 0; i-- {
		out = append(out, today.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return out
}

// --- Markdown export --------------------------------------------------------

func (s *Server) handleExportInterview(w http.ResponseWriter, r *http.Request) {
	iv, ok := s.loadOwnedInterview(w, r)
	if !ok {
		return
	}
	turns, err := s.store.ListInterviewTurns(r.Context(), iv.ID)
	if err != nil {
		storeError(w, err)
		return
	}
	md := renderInterviewMarkdown(iv, turns)
	// RFC 5987 keeps the Chinese title intact in modern browsers while the
	// plain `filename` fallback stays ASCII — a non-ASCII byte in a header
	// value is a protocol error, so both forms are needed.
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="interview-`+iv.ID+`.md"; `+
			`filename*=UTF-8''`+urlEscape(iv.Title)+`.md`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(md))
}

func renderInterviewMarkdown(iv *store.Interview, turns []store.InterviewTurn) string {
	var b strings.Builder
	rep := parseReport(iv.ReportJSON)
	title := iv.Title
	if title == "" {
		title = iv.Role
	}

	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "- **岗位**：%s\n", iv.Role)
	fmt.Fprintf(&b, "- **级别**：%s\n", interview.LabelOf("level", iv.Level))
	fmt.Fprintf(&b, "- **面试类型**：%s\n", interview.LabelOf("type", iv.InterviewType))
	fmt.Fprintf(&b, "- **难度**：%s\n", interview.LabelOf("difficulty", iv.Difficulty))
	fmt.Fprintf(&b, "- **状态**：%s\n", interview.LabelOf("status", iv.Status))
	fmt.Fprintf(&b, "- **生成时间**：%s\n", iv.CreatedAt.UTC().Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(&b, "- **面试官模型**：%s（%s）\n", iv.Model, iv.Engine)
	if iv.DurationSec > 0 {
		fmt.Fprintf(&b, "- **用时**：%d 分 %d 秒\n", iv.DurationSec/60, iv.DurationSec%60)
	}
	b.WriteString("\n")

	if rep != nil {
		fmt.Fprintf(&b, "## 综合评分：%.1f / 100\n\n", rep.OverallScore)
		fmt.Fprintf(&b, "- 预测 HR 满意度：%.0f%%\n", rep.HRSatisfaction)
		fmt.Fprintf(&b, "- 结论：**%s**\n\n", interview.LabelOf("recommendation", rep.Recommendation))
		if rep.RecommendationReason != "" {
			fmt.Fprintf(&b, "> %s\n\n", rep.RecommendationReason)
		}
		b.WriteString("| 维度 | 得分 | 权重 | 评价 |\n|---|---|---|---|\n")
		for _, d := range rep.Dimensions {
			fmt.Fprintf(&b, "| %s | %.0f | %.0f%% | %s |\n",
				d.Label, d.Score, d.Weight*100, mdEscape(d.Comment))
		}
		b.WriteString("\n")
		if rep.Summary != "" {
			b.WriteString("## 总评\n\n")
			b.WriteString(rep.Summary)
			b.WriteString("\n\n")
		}
		b.WriteString(mdList("优势", rep.Strengths))
		b.WriteString(mdList("短板", rep.Weaknesses))
		b.WriteString(mdList("改进建议", rep.Suggestions))
		b.WriteString(mdList("简历改进建议", rep.ResumeSuggestions))
		b.WriteString(mdList("面试策略", rep.InterviewStrategies))
		if len(rep.LearningPlan) > 0 {
			b.WriteString("## 学习计划\n\n")
			for _, l := range rep.LearningPlan {
				fmt.Fprintf(&b, "### %s\n\n- 为什么：%s\n- 怎么做：%s\n\n", l.Topic, l.Why, l.How)
			}
		}
		if len(rep.Highlights) > 0 {
			b.WriteString("## 高光回答\n\n")
			for _, h := range rep.Highlights {
				fmt.Fprintf(&b, "> %s\n\n", mdEscape(h))
			}
		}
		b.WriteString(mdList("风险提示", rep.Risks))
	}

	b.WriteString("## 逐题记录\n\n")
	for i := range turns {
		t := &turns[i]
		kind := "题目"
		if t.Kind == interview.KindFollowup {
			kind = "追问"
		}
		fmt.Fprintf(&b, "### %d. [%s] %s\n\n", t.Seq, kind, t.Question)
		if g := parseGrade(t.GradeJSON); g != nil {
			fmt.Fprintf(&b, "**得分：%.0f / 100**（%s）\n\n",
				g.Score, interview.LabelOf("verdict", g.Verdict))
			if len(g.Dimensions) > 0 {
				keys := make([]string, 0, len(g.Dimensions))
				for k := range g.Dimensions {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				parts := make([]string, 0, len(keys))
				for _, k := range keys {
					parts = append(parts, fmt.Sprintf("%s %.0f", k, g.Dimensions[k]))
				}
				fmt.Fprintf(&b, "维度得分：%s\n\n", strings.Join(parts, " · "))
			}
		}
		b.WriteString("**你的回答**\n\n")
		if strings.TrimSpace(t.Answer) == "" {
			b.WriteString("_（未作答）_\n\n")
		} else {
			for _, line := range strings.Split(t.Answer, "\n") {
				fmt.Fprintf(&b, "> %s\n", line)
			}
			b.WriteString("\n")
		}
		if t.Feedback != "" {
			b.WriteString("**点评**\n\n")
			b.WriteString(t.Feedback)
			b.WriteString("\n\n")
		}
		if g := parseGrade(t.GradeJSON); g != nil && g.Reference != "" {
			fmt.Fprintf(&b, "**参考要点**：%s\n\n", mdEscape(g.Reference))
		}
		b.WriteString("---\n\n")
	}
	b.WriteString("\n_由 Dream Interviewer 生成_\n")
	return b.String()
}

func mdList(heading string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", heading)
	for _, it := range items {
		fmt.Fprintf(&b, "- %s\n", mdEscape(it))
	}
	b.WriteString("\n")
	return b.String()
}

// mdEscape keeps model/user text from breaking out of a Markdown table
// cell or list item.
func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}

// urlEscape percent-encodes a filename for the RFC 5987 form.
func urlEscape(s string) string {
	if strings.TrimSpace(s) == "" {
		return "interview"
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
