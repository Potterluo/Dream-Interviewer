// Package interview is the AI interviewer's brain: it turns an
// interview "spec" (role, level, résumé, JD) into a question plan, grades
// each answer on named dimensions, and rolls the whole session up into a
// report.
//
// Two interchangeable implementations sit behind the Engine:
//
//   - the LLM path (internal/llm) — adaptive, résumé-aware, writes prose
//   - the offline rubric (bank.go) — deterministic, keyword-driven
//
// The offline path is not a placeholder: it is what makes the product
// work with no API key, no network and no account, and it is what the
// tests assert against. Whenever an LLM stage fails the engine degrades
// to it and reports that honestly in Source, so the UI never shows a
// blank screen and never silently pretends a model answered.
package interview

// Interview lifecycle. A draft has no plan yet; in_progress has an open
// question; completed has a report; aborted is a draft the user gave up
// on (kept so the analytics page can count give-ups).
const (
	StatusDraft      = "draft"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusAborted    = "aborted"
)

// Seniority levels. Stored as these slugs; the UI renders Label().
const (
	LevelJunior = "junior"
	LevelMid    = "mid"
	LevelSenior = "senior"
	LevelExpert = "expert"
)

// Interview types. The type decides which dimension set the interview is
// scored on (see DimensionsFor).
const (
	TypeTech         = "tech"
	TypeBehavior     = "behavior"
	TypeMixed        = "mixed"
	TypeSystemDesign = "system_design"
)

// Difficulty biases question selection and the offline grader's
// expectations.
const (
	DifficultyEasy   = "easy"
	DifficultyNormal = "normal"
	DifficultyHard   = "hard"
)

// Prompt/output languages.
const (
	LanguageZH = "zh"
	LanguageEN = "en"
)

// A turn is either a planned question or an adaptive follow-up.
const (
	KindQuestion = "question"
	KindFollowup = "followup"
)

// Per-answer verdicts (cheap to branch on in the UI).
const (
	VerdictStrong = "strong"
	VerdictOK     = "ok"
	VerdictWeak   = "weak"
)

// Whole-interview hiring recommendations, best to worst.
const (
	RecStrongHire = "strong_hire"
	RecHire       = "hire"
	RecMaybe      = "maybe"
	RecNoHire     = "no_hire"
)

// Which implementation produced a result.
const (
	EngineLLM     = "llm"
	EngineOffline = "offline"
)

// Limits the API and the engine both enforce.
const (
	MinQuestions = 3
	MaxQuestions = 15
)

// Dimension keys. Kept as constants because they cross the wire, get
// persisted inside plan/report JSON, and are used as prompt vocabulary.
const (
	DimAccuracy       = "accuracy"        // 技术准确性
	DimDepth          = "depth"           // 技术深度
	DimProblemSolving = "problem_solving" // 问题解决
	DimCommunication  = "communication"   // 沟通表达
	DimSituational    = "situational"     // 情境应对
	DimTeamwork       = "teamwork"        // 团队协作
	DimSelfAwareness  = "self_awareness"  // 自我认知
	DimProfessional   = "professional"    // 专业能力
	DimLogic          = "logic"           // 逻辑思维
	DimFit            = "fit"             // 岗位匹配
)

// Spec is the input to a plan: everything the interviewer knows before
// asking the first question.
type Spec struct {
	Role          string
	Level         string
	Type          string
	Language      string
	Difficulty    string
	QuestionCount int
	Resume        string
	JD            string
}

// Normalize fills defaults, validates enum-ish fields against the known
// values, and clamps the question count. Unknown values fall back to the
// default rather than erroring: a typo in a form field should not 400.
func (s Spec) Normalize() Spec {
	out := s
	if out.Role = trim(out.Role); out.Role == "" {
		out.Role = "软件工程师"
	}
	out.Level = oneOf(out.Level, []string{LevelJunior, LevelMid, LevelSenior, LevelExpert}, LevelMid)
	out.Type = oneOf(out.Type, []string{TypeTech, TypeBehavior, TypeMixed, TypeSystemDesign}, TypeMixed)
	out.Language = oneOf(out.Language, []string{LanguageZH, LanguageEN}, LanguageZH)
	out.Difficulty = oneOf(out.Difficulty, []string{DifficultyEasy, DifficultyNormal, DifficultyHard}, DifficultyNormal)
	if out.QuestionCount < MinQuestions {
		out.QuestionCount = MinQuestions
	}
	if out.QuestionCount > MaxQuestions {
		out.QuestionCount = MaxQuestions
	}
	return out
}

// Dimension is one scoring axis with a weight that sums to 1.0 across
// the interview's dimension set.
type Dimension struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Weight float64 `json:"weight"`
	Desc   string  `json:"desc,omitempty"`
}

// PlannedQuestion is one question the interviewer intends to ask.
type PlannedQuestion struct {
	Seq       int      `json:"seq"`
	Dimension string   `json:"dimension"`
	Question  string   `json:"question"`
	Intent    string   `json:"intent"`
	Weight    float64  `json:"weight"`
	FollowUps []string `json:"followUps,omitempty"`
	Reference string   `json:"reference,omitempty"`
	// Keywords are the terms a strong answer covers. Only the built-in
	// bank populates them; they drive the deterministic grader's coverage
	// signal when no model is available.
	Keywords []string `json:"keywords,omitempty"`
}

// Plan is the whole question script plus the rubric it will be scored on.
type Plan struct {
	Dimensions []Dimension       `json:"dimensions"`
	Questions  []PlannedQuestion `json:"questions"`
	Rationale  string            `json:"rationale,omitempty"`
}

// Grade is the verdict on one answer.
type Grade struct {
	Score      float64            `json:"score"`
	Dimensions map[string]float64 `json:"dimensions"`
	Strengths  []string           `json:"strengths"`
	Weaknesses []string           `json:"weaknesses"`
	Feedback   string             `json:"feedback"`
	Reference  string             `json:"reference"`
	FollowUp   string             `json:"followUp"`
	Verdict    string             `json:"verdict"`
}

// DimensionScore is one axis of the final report.
type DimensionScore struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Score   float64 `json:"score"`
	Weight  float64 `json:"weight"`
	Comment string  `json:"comment,omitempty"`
}

// LearningItem is one concrete "go study this" entry in the report.
type LearningItem struct {
	Topic string `json:"topic"`
	Why   string `json:"why"`
	How   string `json:"how"`
}

// Report is the deliverable at the end of an interview.
//
// Beyond the score roll-up it carries the two artifacts candidates
// actually act on — how to fix the résumé and how to fix the interview —
// plus the reference project's "predicted HR satisfaction" signal,
// which is the number a nervous candidate scrolls to first.
type Report struct {
	OverallScore         float64          `json:"overallScore"`
	HRSatisfaction       float64          `json:"hrSatisfaction"`
	Dimensions           []DimensionScore `json:"dimensions"`
	Summary              string           `json:"summary"`
	Strengths            []string         `json:"strengths"`
	Weaknesses           []string         `json:"weaknesses"`
	Suggestions          []string         `json:"suggestions"`
	ResumeSuggestions    []string         `json:"resumeSuggestions"`
	InterviewStrategies  []string         `json:"interviewStrategies"`
	LearningPlan         []LearningItem   `json:"learningPlan"`
	Recommendation       string           `json:"recommendation"`
	RecommendationReason string           `json:"recommendationReason"`
	Highlights           []string         `json:"highlights"`
	Risks                []string         `json:"risks"`
}

// Normalize guarantees the non-null array fields the frontend maps over
// and keeps the two bucket-derived fields consistent with the score, so
// a model that omits them cannot produce a report with a blank verdict.
func (r *Report) Normalize(dims []Dimension) {
	if r.Recommendation == "" {
		r.Recommendation = RecommendationForScore(r.OverallScore)
	}
	if r.HRSatisfaction <= 0 {
		r.HRSatisfaction = r.OverallScore
	}
	r.HRSatisfaction = round1(clamp(r.HRSatisfaction, 0, 100))
	r.OverallScore = round1(clamp(r.OverallScore, 0, 100))
	r.Strengths = strSlice(uniqStrings(r.Strengths))
	r.Weaknesses = strSlice(uniqStrings(r.Weaknesses))
	r.Suggestions = strSlice(uniqStrings(r.Suggestions))
	r.ResumeSuggestions = strSlice(uniqStrings(r.ResumeSuggestions))
	r.InterviewStrategies = strSlice(uniqStrings(r.InterviewStrategies))
	r.Highlights = strSlice(uniqStrings(r.Highlights))
	r.Risks = strSlice(uniqStrings(r.Risks))
	if r.LearningPlan == nil {
		r.LearningPlan = []LearningItem{}
	}
	if r.Dimensions == nil {
		r.Dimensions = []DimensionScore{}
	}
	// Fill label/weight from the rubric when the model omitted them, and
	// drop dimensions the rubric does not define.
	fixed := make([]DimensionScore, 0, len(r.Dimensions))
	for _, ds := range r.Dimensions {
		d, ok := DimensionByKey(dims, ds.Key)
		if !ok {
			continue
		}
		ds.Label = d.Label
		ds.Weight = d.Weight
		ds.Score = round1(clamp(ds.Score, 0, 100))
		fixed = append(fixed, ds)
	}
	r.Dimensions = fixed
}

// TurnBrief is the compact history handed to the LLM stages: enough
// context to ask a coherent follow-up without re-sending full grades.
type TurnBrief struct {
	Seq       int
	Kind      string
	Dimension string
	Question  string
	Answer    string
	Score     float64
	Feedback  string
	// DimScores carries the per-dimension breakdown of this turn's grade.
	// The report writer needs it to average each dimension across the
	// interview; the prompt transcript intentionally ignores it (it is
	// heavy and the model already sees the score).
	DimScores map[string]float64
}

// Source says which implementation answered, and why it degraded when it
// did. Handlers surface this to the UI.
type Source struct {
	Engine string `json:"engine"` // llm | offline
	Model  string `json:"model"`
	Note   string `json:"note,omitempty"` // fallback reason, when offline
	// Transient marks a result the engine deliberately did not produce (a
	// contentless answer routed straight to the rubric). Callers should
	// report it honestly but must NOT let it overwrite the interview's
	// recorded engine, or one 2-character answer would make an otherwise
	// model-driven session look like it ran on the fallback.
	Transient bool `json:"transient,omitempty"`
}

// GradeInput is everything the grader needs.
type GradeInput struct {
	Spec      Spec
	Plan      Plan
	Question  PlannedQuestion
	Kind      string
	Answer    string
	History   []TurnBrief
	Answered  int // turns already answered, for follow-up budgeting
	FollowUps int // follow-ups already spent in this interview
}

// ReportInput is everything the report writer needs.
type ReportInput struct {
	Spec  Spec
	Plan  Plan
	Turns []TurnBrief
}

// --- label tables -----------------------------------------------------------

// labelSets maps each enum to its Chinese display label. The UI fetches
// these from GET /api/interview/presets so copy lives in exactly one
// place, and the prompts use them so the model sees consistent wording.
var labelSets = map[string][]Label{
	"level": {
		{Value: LevelJunior, Label: "初级"},
		{Value: LevelMid, Label: "中级"},
		{Value: LevelSenior, Label: "高级"},
		{Value: LevelExpert, Label: "资深/专家"},
	},
	"type": {
		{Value: TypeTech, Label: "技术面试"},
		{Value: TypeBehavior, Label: "行为面试"},
		{Value: TypeMixed, Label: "综合面试"},
		{Value: TypeSystemDesign, Label: "系统设计"},
	},
	"difficulty": {
		{Value: DifficultyEasy, Label: "简单"},
		{Value: DifficultyNormal, Label: "常规"},
		{Value: DifficultyHard, Label: "困难"},
	},
	"language": {
		{Value: LanguageZH, Label: "中文"},
		{Value: LanguageEN, Label: "English"},
	},
	"status": {
		{Value: StatusDraft, Label: "草稿"},
		{Value: StatusInProgress, Label: "进行中"},
		{Value: StatusCompleted, Label: "已完成"},
		{Value: StatusAborted, Label: "已放弃"},
	},
	"recommendation": {
		{Value: RecStrongHire, Label: "强烈推荐"},
		{Value: RecHire, Label: "推荐录用"},
		{Value: RecMaybe, Label: "待定"},
		{Value: RecNoHire, Label: "不推荐"},
	},
	"verdict": {
		{Value: VerdictStrong, Label: "优秀"},
		{Value: VerdictOK, Label: "合格"},
		{Value: VerdictWeak, Label: "偏弱"},
	},
}

// Label is one enum value with its display name.
type Label struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Labels returns a copy of the label table for a set name. The UI reads
// these; unknown names yield an empty slice.
func Labels(set string) []Label {
	src := labelSets[set]
	out := make([]Label, len(src))
	copy(out, src)
	return out
}

// AllLabels returns every label set, for GET /api/interview/presets.
func AllLabels() map[string][]Label {
	out := make(map[string][]Label, len(labelSets))
	for k := range labelSets {
		out[k] = Labels(k)
	}
	return out
}

// LabelOf resolves one value to its display label, falling back to the
// raw value so an unexpected string still renders.
func LabelOf(set, value string) string {
	for _, l := range labelSets[set] {
		if l.Value == value {
			return l.Label
		}
	}
	return value
}

// VerdictForScore buckets an answer score into a verdict.
func VerdictForScore(score float64) string {
	switch {
	case score >= 80:
		return VerdictStrong
	case score >= 60:
		return VerdictOK
	default:
		return VerdictWeak
	}
}

// RecommendationForScore buckets an overall score into a recommendation.
func RecommendationForScore(score float64) string {
	switch {
	case score >= 85:
		return RecStrongHire
	case score >= 72:
		return RecHire
	case score >= 58:
		return RecMaybe
	default:
		return RecNoHire
	}
}
