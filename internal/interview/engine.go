package interview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Potterluo/dream-interviewer/internal/llm"
)

// engine.go is the orchestrator. Each stage follows the same contract:
//
//  1. if an LLM is configured, try it (streaming its prose to the caller)
//  2. if that fails for any reason other than the caller cancelling,
//     degrade to the deterministic offline rubric and say so in Source
//
// The cancellation carve-out matters: when the candidate presses Stop,
// `ctx` is done and the honest answer is "you stopped me", not "here is
// a heuristic score you did not ask for".
type Engine struct {
	client *llm.Client
}

// NewEngine wraps a client. A nil client yields a working offline-only
// engine, which is what tests and zero-config installs use.
func NewEngine(client *llm.Client) *Engine {
	if client == nil {
		client = llm.New(llm.Config{})
	}
	return &Engine{client: client}
}

// Client exposes the underlying transport (the engine panel reports on it).
func (e *Engine) Client() *llm.Client { return e.client }

// Ready reports whether a real model is configured.
func (e *Engine) Ready() bool { return e.client.Configured() }

// --- plan -------------------------------------------------------------------

// BuildPlan produces the question script. The plan stage is machine-only
// (bare JSON), so there is nothing to stream token-by-token; instead the
// narrative callback fires once, with the plan's rationale, so the UI has
// something to show at the moment the questions appear.
func (e *Engine) BuildPlan(ctx context.Context, spec Spec, visible func(string) error) (Plan, Source, error) {
	spec = spec.Normalize()

	if e.client.Configured() {
		plan, err := e.planLLM(ctx, spec)
		if err == nil {
			if visible != nil && plan.Rationale != "" {
				_ = visible(plan.Rationale)
			}
			return plan, Source{Engine: EngineLLM, Model: e.client.Model()}, nil
		}
		if cancelled(ctx, err) {
			return Plan{}, Source{}, err
		}
		plan = offlinePlan(spec)
		if visible != nil {
			_ = visible(plan.Rationale)
		}
		return plan, Source{Engine: EngineOffline, Model: offlineModelName, Note: err.Error()}, nil
	}

	plan := offlinePlan(spec)
	if visible != nil {
		_ = visible(plan.Rationale)
	}
	return plan, Source{Engine: EngineOffline, Model: offlineModelName, Note: "未配置模型服务，使用内置题库"}, nil
}

func (e *Engine) planLLM(ctx context.Context, spec Spec) (Plan, error) {
	var plan Plan
	if _, err := e.client.ChatJSON(ctx, PlanMessages(spec), &plan); err != nil {
		return Plan{}, err
	}
	if err := normalizePlan(&plan, spec); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

// normalizePlan validates and repairs a model-authored plan: unknown
// dimension keys are dropped, weights renormalised, questions renumbered
// and topped up/truncated to the requested count. A plan that cannot be
// salvaged (no usable questions) is an error, which triggers the offline
// path — better a real question bank than five empty strings.
func normalizePlan(plan *Plan, spec Spec) error {
	dims := DimensionsFor(spec)
	if len(plan.Dimensions) > 0 {
		kept := make([]Dimension, 0, len(plan.Dimensions))
		for _, d := range plan.Dimensions {
			if def, ok := DimensionByKey(dims, d.Key); ok {
				// Trust the rubric for label/weight, keep any model desc.
				def.Desc = firstNonEmpty(d.Desc, def.Desc)
				kept = append(kept, def)
			}
		}
		if len(kept) > 0 {
			dims = NormalizeDimensions(kept)
		}
	}
	dims = NormalizeDimensions(dims)
	plan.Dimensions = dims

	valid := make(map[string]bool, len(dims))
	for _, d := range dims {
		valid[d.Key] = true
	}

	questions := make([]PlannedQuestion, 0, len(plan.Questions))
	for _, q := range plan.Questions {
		q.Question = strings.TrimSpace(q.Question)
		if q.Question == "" {
			continue
		}
		if !valid[q.Dimension] {
			q.Dimension = dims[0].Key
		}
		if q.Weight <= 0 {
			q.Weight = 1
		}
		q.Intent = strings.TrimSpace(q.Intent)
		q.Reference = strings.TrimSpace(q.Reference)
		q.FollowUps = uniqStrings(q.FollowUps)
		if len(q.FollowUps) > 2 {
			q.FollowUps = q.FollowUps[:2]
		}
		questions = append(questions, q)
	}
	if len(questions) == 0 {
		return errors.New("模型未返回有效题目")
	}
	if len(questions) > spec.QuestionCount {
		questions = questions[:spec.QuestionCount]
	}
	for i := range questions {
		questions[i].Seq = i + 1
	}
	plan.Questions = questions
	plan.Rationale = strings.TrimSpace(plan.Rationale)
	return nil
}

// --- grade ------------------------------------------------------------------

// TrivialAnswer reports whether an answer is too short to carry anything
// assessable — "a", "嗯", "1".
//
// This is a gate on CALLING THE MODEL, not on scoring: asked to evaluate a
// contentless answer, a model reliably invents a critique (observed: a
// one-character answer produced a paragraph about rubric design and
// overfitting that had nothing to do with the question). Refusing to ask
// is both cheaper and more honest — the deterministic rule engine already
// has an honest answer for this case.
func TrivialAnswer(answer string) bool {
	return len([]rune(strings.TrimSpace(answer))) < 4
}

// GradeAnswer grades one answer and yields the candidate-facing 点评 as
// streamed deltas.
func (e *Engine) GradeAnswer(ctx context.Context, in GradeInput, visible func(string) error) (Grade, Source, error) {
	in.Spec = in.Spec.Normalize()
	if len(in.Plan.Dimensions) == 0 {
		in.Plan.Dimensions = DimensionsFor(in.Spec)
	}

	if TrivialAnswer(in.Answer) {
		grade := offlineGrade(in)
		if visible != nil {
			_ = visible(grade.Feedback)
		}
		// Transient: honest about this answer, but it must not relabel the
		// whole interview as offline-run.
		return grade, Source{Engine: EngineOffline, Model: offlineModelName,
			Note: "回答过短，没有可评估的内容，本次未调用模型", Transient: true}, nil
	}

	if e.client.Configured() {
		split := newStreamSplitter(jsonDelimiter, visible)
		raw, err := e.client.ChatStream(ctx, GradeMessages(in), split.Push)
		if err == nil {
			var grade Grade
			if decErr := decodeSplitReply(split, raw, &grade); decErr == nil {
				normalizeGrade(&grade, in)
				return grade, Source{Engine: EngineLLM, Model: e.client.Model()}, nil
			} else {
				err = decErr
			}
		}
		if cancelled(ctx, err) {
			return Grade{}, Source{}, err
		}
		grade := offlineGrade(in)
		// Only re-narrate when the model never got to say anything,
		// otherwise the candidate would read two different verdicts.
		if !split.emitted() && visible != nil {
			_ = visible(grade.Feedback)
		}
		return grade, Source{Engine: EngineOffline, Model: offlineModelName, Note: err.Error()}, nil
	}

	grade := offlineGrade(in)
	if visible != nil {
		_ = visible(grade.Feedback)
	}
	return grade, Source{Engine: EngineOffline, Model: offlineModelName, Note: "未配置模型服务，使用内置评分规则"}, nil
}

// normalizeGrade fills in what the model left out and drops what the
// rubric does not recognise, so downstream code (and the radar chart)
// always has a complete set of dimensions.
func normalizeGrade(g *Grade, in GradeInput) {
	dims := in.Plan.Dimensions
	if len(dims) == 0 {
		dims = DimensionsFor(in.Spec)
	}
	scores := make(map[string]float64, len(dims))
	for _, d := range dims {
		if v, ok := g.Dimensions[d.Key]; ok {
			scores[d.Key] = round1(clamp(v, 0, 100))
		}
	}
	if g.Score <= 0 && len(scores) > 0 {
		g.Score = WeightedScore(dims, scores)
	}
	g.Score = round1(clamp(g.Score, 0, 100))
	// An omitted dimension falls back to the overall score rather than 0:
	// "not assessed" must not read as "failed".
	if g.Score > 0 {
		for _, d := range dims {
			if _, ok := scores[d.Key]; !ok {
				scores[d.Key] = g.Score
			}
		}
	}
	g.Dimensions = scores
	g.Strengths = strSlice(uniqStrings(g.Strengths))
	g.Weaknesses = strSlice(uniqStrings(g.Weaknesses))
	g.Feedback = strings.TrimSpace(g.Feedback)
	g.Reference = strings.TrimSpace(g.Reference)
	g.FollowUp = strings.TrimSpace(g.FollowUp)
	if g.Verdict == "" {
		g.Verdict = VerdictForScore(g.Score)
	}
	// A skipped answer gets no follow-up: there is nothing to dig into.
	if strings.TrimSpace(in.Answer) == "" {
		g.FollowUp = ""
		g.Verdict = VerdictWeak
	}
}

// --- report -----------------------------------------------------------------

// BuildReport writes the end-of-interview report, streaming the 总评.
func (e *Engine) BuildReport(ctx context.Context, in ReportInput, visible func(string) error) (Report, Source, error) {
	in.Spec = in.Spec.Normalize()
	if len(in.Plan.Dimensions) == 0 {
		in.Plan.Dimensions = DimensionsFor(in.Spec)
	}

	if e.client.Configured() {
		split := newStreamSplitter(jsonDelimiter, visible)
		raw, err := e.client.ChatStream(ctx, ReportMessages(in), split.Push)
		if err == nil {
			var report Report
			if decErr := decodeSplitReply(split, raw, &report); decErr == nil {
				normalizeReport(&report, in)
				return report, Source{Engine: EngineLLM, Model: e.client.Model()}, nil
			} else {
				err = decErr
			}
		}
		if cancelled(ctx, err) {
			return Report{}, Source{}, err
		}
		report := offlineReport(in)
		if !split.emitted() && visible != nil {
			_ = visible(report.Summary)
		}
		return report, Source{Engine: EngineOffline, Model: offlineModelName, Note: err.Error()}, nil
	}

	report := offlineReport(in)
	if visible != nil {
		_ = visible(report.Summary)
	}
	return report, Source{Engine: EngineOffline, Model: offlineModelName, Note: "未配置模型服务，使用内置评分规则"}, nil
}

// normalizeReport makes the model's report self-consistent: the overall
// score is recomputed from the per-dimension scores (the model's own
// arithmetic is not trustworthy), and anything missing is derived.
func normalizeReport(r *Report, in ReportInput) {
	dims := in.Plan.Dimensions
	if len(dims) == 0 {
		dims = DimensionsFor(in.Spec)
	}

	byKey := make(map[string]float64, len(r.Dimensions))
	for _, ds := range r.Dimensions {
		byKey[ds.Key] = ds.Score
	}
	// Recompute both roll-ups from the answers themselves when the model
	// omitted them, and always prefer the model's dimension scores for
	// the weighted total.
	if len(byKey) > 0 {
		computed := WeightedScore(dims, byKey)
		if r.OverallScore <= 0 {
			r.OverallScore = computed
		}
	}
	if r.OverallScore <= 0 {
		r.OverallScore = averageTurnScore(in.Turns)
	}
	r.Normalize(dims)

	// A report with no dimension rows at all (model returned nothing
	// usable) is backfilled from the answer scores so the radar renders.
	if len(r.Dimensions) == 0 {
		for _, d := range dims {
			r.Dimensions = append(r.Dimensions, DimensionScore{
				Key: d.Key, Label: d.Label, Weight: d.Weight, Score: r.OverallScore,
			})
		}
	}
	r.Summary = strings.TrimSpace(r.Summary)
	r.RecommendationReason = strings.TrimSpace(r.RecommendationReason)
	if r.Summary == "" {
		r.Summary = fallbackSummary(r)
	}
}

// --- engine self-test -------------------------------------------------------

// TestResult is the outcome of POST /api/interview/engine/test.
type TestResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Model     string `json:"model"`
	Reply     string `json:"reply"`
	Error     string `json:"error,omitempty"`
}

// Test makes one tiny live call to prove the endpoint, key and model all
// work. An optional model override lets the UI try a different model
// without a restart (the config itself is env-only by design).
func (e *Engine) Test(ctx context.Context, model string) TestResult {
	probe := e.client
	if strings.TrimSpace(model) != "" && model != e.client.Model() {
		cfg := e.client.Config()
		cfg.Model = model
		probe = llm.New(cfg)
	}
	res := TestResult{Model: probe.Model()}
	if !probe.Configured() {
		res.Error = "未配置模型服务。请设置 APP_LLM_PROVIDER，或 APP_LLM_BASE_URL + APP_LLM_MODEL。"
		return res
	}
	started := nowMs()
	reply, err := probe.Chat(ctx, TestMessages())
	res.LatencyMs = nowMs() - started
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.OK = true
	res.Reply = TrimTo(strings.TrimSpace(reply), 200)
	return res
}

// --- helpers ----------------------------------------------------------------

const offlineModelName = "内置题库与评分规则"

// cancelled reports whether err is really "the caller gave up".
func cancelled(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// decodeSplitReply pulls the JSON half out of a two-part reply and
// unmarshals it. When the model forgot the delimiter we fall back to
// scanning the whole reply, which is why the marker is an optimisation
// for streaming, not a correctness requirement.
func decodeSplitReply(split *streamSplitter, raw string, out any) error {
	for _, candidate := range []string{split.JSONText(), raw} {
		payload, ok := llm.ExtractJSON(candidate)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(payload), out); err != nil {
			return fmt.Errorf("模型返回的 JSON 无法解析: %w", err)
		}
		return nil
	}
	return errors.New("模型返回中未找到 JSON 结构")
}

func averageTurnScore(turns []TurnBrief) float64 {
	if len(turns) == 0 {
		return 0
	}
	total, n := 0.0, 0
	for _, t := range turns {
		total += t.Score
		n++
	}
	if n == 0 {
		return 0
	}
	return round1(total / float64(n))
}

func fallbackSummary(r *Report) string {
	return fmt.Sprintf("本场面试综合得分 %.1f 分。%s", r.OverallScore,
		LabelOf("recommendation", r.Recommendation))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// --- stream splitter --------------------------------------------------------

// streamSplitter separates a two-part model reply into the prose the
// candidate should see live and the JSON tail that must not be shown.
//
// It holds back the last len(delimiter)-1 bytes of undecided text so a
// delimiter split across two SSE frames is still detected; without that,
// a chunk boundary in the middle of "<<<JSON>>>" would leak the marker
// (and the JSON after it) into the visible transcript.
//
// The hold-back is measured in BYTES, so the cut has to be pulled back to
// a rune boundary. Chinese text is 3 bytes per character, which means a
// naive byte cut lands inside a character most of the time — and because
// each emitted chunk is marshalled to JSON, the dangling bytes came out as
// U+FFFD replacement glyphs in the live stream. (The stored feedback stayed
// correct because `raw` is never cut, which is exactly why this was easy to
// miss: the report was right and only the streaming panel looked broken.)
type streamSplitter struct {
	delim   string
	visible func(string) error

	raw     strings.Builder
	pending string
	jsonBuf strings.Builder
	seen    bool
	started bool
	narr    strings.Builder
}

func newStreamSplitter(delim string, visible func(string) error) *streamSplitter {
	return &streamSplitter{delim: delim, visible: visible}
}

// Push consumes one delta.
func (s *streamSplitter) Push(delta string) error {
	s.raw.WriteString(delta)

	if s.seen {
		s.jsonBuf.WriteString(delta)
		return nil
	}

	s.pending += delta
	if idx := strings.Index(s.pending, s.delim); idx >= 0 {
		// Safe without adjustment: the delimiter is ASCII, so a match can
		// only start on a rune boundary.
		head := s.pending[:idx]
		s.jsonBuf.WriteString(s.pending[idx+len(s.delim):])
		s.pending = ""
		s.seen = true
		return s.emit(head)
	}

	if keep := len(s.delim) - 1; len(s.pending) > keep {
		cut := len(s.pending) - keep
		// Walk back off any continuation byte so we never split a rune.
		// A rune is at most 4 bytes and keep is 9, so cut always ends up
		// positive here; the guard is for safety if the delimiter shrinks.
		for cut > 0 && !utf8.RuneStart(s.pending[cut]) {
			cut--
		}
		if cut == 0 {
			return nil
		}
		emit := s.pending[:cut]
		s.pending = s.pending[cut:]
		return s.emit(emit)
	}
	return nil
}

func (s *streamSplitter) emit(chunk string) error {
	// Trim blank lines ONCE, before any prose has gone out, so a reply that
	// opens with "\n\n" does not begin with empty lines. Trimming per chunk
	// instead would eat every newline that happened to start a provider
	// delta, silently flattening the streamed Markdown.
	if !s.started {
		chunk = strings.TrimLeft(chunk, "\r\n")
	}
	if chunk == "" {
		return nil
	}
	s.started = true
	s.narr.WriteString(chunk)
	if s.visible != nil {
		return s.visible(chunk)
	}
	return nil
}

// JSONText is the text the JSON lives in: everything after the delimiter
// when it was found, otherwise the whole reply.
func (s *streamSplitter) JSONText() string {
	if s.seen {
		return s.jsonBuf.String()
	}
	return s.raw.String()
}

// emitted reports whether any prose reached the caller.
func (s *streamSplitter) emitted() bool { return s.narr.Len() > 0 }
