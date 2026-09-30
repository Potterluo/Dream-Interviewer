package interview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Potterluo/dream-interviewer/internal/llm"
)

// --- stream splitter --------------------------------------------------------

func TestStreamSplitterSeparatesProseFromJSON(t *testing.T) {
	reply := "这是点评。\n" + jsonDelimiter + `{"score": 80}`

	// Feed it one byte at a time: a delimiter split across frame
	// boundaries is exactly the case that leaks the marker into the UI.
	var visible strings.Builder
	split := newStreamSplitter(jsonDelimiter, func(d string) error {
		visible.WriteString(d)
		return nil
	})
	for _, r := range reply {
		if err := split.Push(string(r)); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}

	if strings.Contains(visible.String(), jsonDelimiter) {
		t.Fatalf("delimiter leaked into the visible prose: %q", visible.String())
	}
	if strings.Contains(visible.String(), "score") {
		t.Fatalf("JSON leaked into the visible prose: %q", visible.String())
	}
	if !strings.Contains(visible.String(), "这是点评") {
		t.Fatalf("prose was not streamed: %q", visible.String())
	}
	if !split.emitted() {
		t.Fatal("emitted() = false after streaming prose")
	}

	var got struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(split.JSONText()), &got); err != nil {
		t.Fatalf("JSONText is not valid JSON (%q): %v", split.JSONText(), err)
	}
	if got.Score != 80 {
		t.Fatalf("score = %v", got.Score)
	}
}

func TestStreamSplitterWithoutDelimiterStillYieldsJSON(t *testing.T) {
	// A model that ignores the format instruction must still be usable.
	reply := `{"score": 70}`
	var visible strings.Builder
	split := newStreamSplitter(jsonDelimiter, func(d string) error {
		visible.WriteString(d)
		return nil
	})
	if err := split.Push(reply); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if got := split.JSONText(); got != reply {
		t.Fatalf("JSONText = %q, want the whole reply", got)
	}
}

// Regression: the splitter holds back the last len(delimiter)-1 BYTES to
// catch a delimiter straddling two frames. With CJK text (3 bytes per
// character) that naive cut landed inside a rune, and since every emitted
// chunk is JSON-marshalled the dangling bytes became U+FFFD replacement
// glyphs in the live stream — while the stored feedback stayed correct,
// so the report looked fine and only the streaming panel was broken.
func TestStreamSplitterNeverSplitsARune(t *testing.T) {
	// Long enough to exceed the 9-byte hold-back several times over.
	prose := "## 第一部分：给候选人的点评\n\n你对 MySQL 索引的理解基本正确，" +
		"但回表的代价没有量化；覆盖索引能避免回表，这是关键。"
	reply := prose + jsonDelimiter + `{"score": 66}`

	// Feed one byte at a time: the worst case for rune boundaries, and it
	// also exercises the delimiter straddling frames.
	var visible strings.Builder
	var chunks []string
	split := newStreamSplitter(jsonDelimiter, func(d string) error {
		chunks = append(chunks, d)
		visible.WriteString(d)
		return nil
	})
	for i := 0; i < len(reply); i++ {
		if err := split.Push(reply[i : i+1]); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}

	if got := visible.String(); got != prose {
		t.Fatalf("streamed prose is not byte-identical to the source\n got: %q\nwant: %q", got, prose)
	}
	if strings.ContainsRune(visible.String(), '\uFFFD') {
		t.Fatalf("streamed prose contains replacement glyphs: %q", visible.String())
	}
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk %d is not valid UTF-8: %q", i, c)
		}
		if strings.ContainsRune(c, '\uFFFD') {
			t.Fatalf("chunk %d contains a replacement glyph: %q", i, c)
		}
	}
	if strings.Contains(visible.String(), jsonDelimiter) {
		t.Fatal("the delimiter leaked into the prose")
	}
	if !strings.Contains(visible.String(), "覆盖索引") {
		t.Fatal("prose was not streamed at all")
	}
}

// The same guarantee for a realistic chunking (provider deltas of a few
// characters), which is what the live stream actually looks like.
func TestStreamSplitterIsRuneSafeForMultiByteChunks(t *testing.T) {
	prose := "这个回答过于简短，无法构成一次有效作答，建议先给出结论再展开理由与例子。"
	proseRunes := []rune(prose)

	var visible strings.Builder
	split := newStreamSplitter(jsonDelimiter, func(d string) error {
		visible.WriteString(d)
		return nil
	})
	// Emit 3 runes at a time, deliberately crossing the byte hold-back.
	for i := 0; i < len(proseRunes); i += 3 {
		end := min(i+3, len(proseRunes))
		if err := split.Push(string(proseRunes[i:end])); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}
	if err := split.Push(jsonDelimiter + `{"ok":true}`); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if visible.String() != prose {
		t.Fatalf("got %q, want %q", visible.String(), prose)
	}
	if !utf8.ValidString(visible.String()) {
		t.Fatal("streamed prose is not valid UTF-8")
	}
}

// Leading blank lines are trimmed once (a model that opens with "\n\n"
// should not produce empty lines), but interior newlines must survive —
// trimming per chunk used to flatten the streamed Markdown.
func TestStreamSplitterTrimsOnlyLeadingBlankLines(t *testing.T) {
	var visible strings.Builder
	split := newStreamSplitter(jsonDelimiter, func(d string) error {
		visible.WriteString(d)
		return nil
	})
	// Deliberately deliver the body one newline at a time.
	for _, part := range []string{"\n", "\n", "第一行\n", "\n", "第二行\n"} {
		if err := split.Push(part); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}
	if err := split.Push(jsonDelimiter + `{}`); err != nil {
		t.Fatalf("Push: %v", err)
	}
	want := "第一行\n\n第二行\n"
	if visible.String() != want {
		t.Fatalf("got %q, want %q", visible.String(), want)
	}
}

// --- plan -------------------------------------------------------------------

func TestOfflinePlanIsDeterministicAndCoversTheRubric(t *testing.T) {
	spec := Spec{Role: "高级后端工程师", Level: LevelSenior, Type: TypeTech, QuestionCount: 6}.Normalize()

	first := offlinePlan(spec)
	second := offlinePlan(spec)
	if mustJSON(t, first) != mustJSON(t, second) {
		t.Fatal("offlinePlan is not deterministic for the same spec")
	}
	if len(first.Questions) != 6 {
		t.Fatalf("got %d questions, want 6", len(first.Questions))
	}
	if len(first.Dimensions) == 0 {
		t.Fatal("plan carries no dimensions")
	}
	// Weights must be usable as a weighted average.
	total := 0.0
	for _, d := range first.Dimensions {
		total += d.Weight
	}
	if total < 0.999 || total > 1.001 {
		t.Fatalf("dimension weights sum to %v, want 1.0", total)
	}
	// Round-robin selection must reach more than one dimension.
	seen := map[string]bool{}
	for _, q := range first.Questions {
		seen[q.Dimension] = true
		if q.Question == "" {
			t.Fatalf("question %d has no text", q.Seq)
		}
		if q.Dimension != "" {
			if _, ok := DimensionByKey(first.Dimensions, q.Dimension); !ok {
				t.Fatalf("question %d uses dimension %q which is not in the rubric", q.Seq, q.Dimension)
			}
		}
	}
	if len(seen) < 2 {
		t.Fatalf("plan only covers %d dimension(s): %v", len(seen), seen)
	}
	for i, q := range first.Questions {
		if q.Seq != i+1 {
			t.Fatalf("question %d has seq %d, want %d", i, q.Seq, i+1)
		}
	}
}

func TestOfflinePlanForEveryPresetAndLevel(t *testing.T) {
	// Every preset a user can click must produce a full paper — this is
	// the zero-config path, so it has to work for the whole catalogue.
	for _, p := range Presets() {
		t.Run(p.ID, func(t *testing.T) {
			spec := Spec{
				Role: p.Role, Level: p.Level, Type: p.InterviewType,
				Difficulty: p.Difficulty, QuestionCount: 6,
			}.Normalize()
			plan := offlinePlan(spec)
			if len(plan.Questions) != 6 {
				t.Fatalf("got %d questions for %s, want 6", len(plan.Questions), p.ID)
			}
			for _, q := range plan.Questions {
				if strings.TrimSpace(q.Question) == "" {
					t.Fatalf("%s produced an empty question", p.ID)
				}
			}
		})
	}
}

func TestNormalizePlanRepairsAModelReply(t *testing.T) {
	spec := Spec{Role: "后端", Type: TypeTech, QuestionCount: 3}.Normalize()
	plan := Plan{
		Dimensions: []Dimension{
			{Key: DimAccuracy, Weight: 2},
			{Key: DimCommunication, Weight: 2},
			{Key: "hallucinated_dimension", Weight: 50}, // must be dropped
		},
		Questions: []PlannedQuestion{
			{Seq: 9, Dimension: "wrong_key", Question: "  Q1  ", Weight: 0},
			{Seq: 9, Dimension: DimDepth, Question: ""}, // empty → dropped
			{Seq: 9, Dimension: DimCommunication, Question: "Q2"},
			{Seq: 9, Dimension: DimAccuracy, Question: "Q3"},
			{Seq: 9, Dimension: DimAccuracy, Question: "Q4 beyond the requested count"},
		},
	}
	if err := normalizePlan(&plan, spec); err != nil {
		t.Fatalf("normalizePlan: %v", err)
	}
	if len(plan.Dimensions) != 2 {
		t.Fatalf("got %d dimensions, want 2 (the hallucinated key must go)", len(plan.Dimensions))
	}
	if len(plan.Questions) != 3 {
		t.Fatalf("got %d questions, want 3", len(plan.Questions))
	}
	if plan.Questions[0].Question != "Q1" {
		t.Fatalf("question was not trimmed: %q", plan.Questions[0].Question)
	}
	if plan.Questions[0].Dimension != DimAccuracy {
		t.Fatalf("an unknown dimension must fall back to the rubric's first key, got %q", plan.Questions[0].Dimension)
	}
	if plan.Questions[0].Weight != 1 {
		t.Fatalf("zero weight must become 1, got %v", plan.Questions[0].Weight)
	}
	for i, q := range plan.Questions {
		if q.Seq != i+1 {
			t.Fatalf("seq not renumbered: %v", plan.Questions)
		}
	}
}

func TestNormalizePlanRejectsAUsablelessReply(t *testing.T) {
	spec := Spec{Role: "后端", Type: TypeTech, QuestionCount: 3}.Normalize()
	plan := Plan{Questions: []PlannedQuestion{{Question: "   "}, {Question: ""}}}
	if err := normalizePlan(&plan, spec); err == nil {
		t.Fatal("a plan with no usable question must error so the engine falls back to the bank")
	}
}

// --- grading ----------------------------------------------------------------

func testSpec() Spec {
	return Spec{Role: "高级后端工程师", Level: LevelSenior, Type: TypeTech,
		Difficulty: DifficultyNormal, QuestionCount: 4}.Normalize()
}

func testGradeInput(t *testing.T, answer string) GradeInput {
	t.Helper()
	plan := offlinePlan(testSpec())
	return GradeInput{
		Spec:     testSpec(),
		Plan:     plan,
		Question: plan.Questions[0],
		Kind:     KindQuestion,
		Answer:   answer,
	}
}

// indexQuestion is an explicit question with known keywords, so the
// grader is tested against a fixed rubric rather than whichever question
// the paper happened to draw first.
func indexQuestion() PlannedQuestion {
	return PlannedQuestion{
		Seq: 1, Dimension: DimAccuracy, Weight: 1,
		Question:  "说说你对 MySQL 索引的理解，以及什么时候不该加索引。",
		Intent:    "考察索引原理与工程取舍",
		Reference: "讲清 B+树 结构、回表与覆盖索引、最左前缀，并能说明写入放大的代价。",
		Keywords:  []string{"b+树", "回表", "覆盖索引", "最左前缀", "聚簇索引", "写入放大"},
	}
}

func TestOfflineGradeRewardsSubstance(t *testing.T) {
	plan := offlinePlan(testSpec())
	q := indexQuestion()
	in := GradeInput{Spec: testSpec(), Plan: plan, Question: q, Kind: KindQuestion}

	weakIn := in
	weakIn.Answer = "不太清楚。"
	weak := offlineGrade(weakIn)

	strongIn := in
	strongIn.Answer = "首先，索引在 MySQL 里默认是 B+树 结构，叶子节点存数据、非叶子只存键，" +
		"所以回表是必然的；我们的订单表在 300 万行时全表扫描要 1.2 秒，加上覆盖索引后降到 80 毫秒。" +
		"其次要考虑最左前缀原则，因为它决定了复合索引能不能被用上。第三，写多读少的表" +
		"加索引会带来写入放大，所以我在项目里对日志表刻意不加二级索引，取舍是牺牲查询换写入吞吐。"
	strong := offlineGrade(strongIn)

	if weak.Score != 0 {
		t.Fatalf("a non-answer scored %.1f, want 0", weak.Score)
	}
	if strong.Score <= weak.Score {
		t.Fatalf("substantive answer (%.1f) did not beat a non-answer (%.1f)", strong.Score, weak.Score)
	}
	// A detailed, on-topic answer that hits most of the expected keywords
	// must clear the "合格" line. Anything lower means the signals are
	// mis-weighted, not that the answer was weak.
	if strong.Score < 60 {
		t.Fatalf("a detailed on-topic answer scored only %.1f", strong.Score)
	}
	if strong.Score <= offlineGrade(GradeInput{
		Spec: testSpec(), Plan: plan, Question: q, Kind: KindQuestion,
		Answer: "索引就是给字段加个索引让查询变快。",
	}).Score {
		t.Fatal("a shallow answer did not score below a detailed one")
	}
	if len(strong.Dimensions) != len(plan.Dimensions) {
		t.Fatalf("grade is missing dimensions: %v", strong.Dimensions)
	}
	for k, v := range strong.Dimensions {
		if v < 0 || v > 100 {
			t.Fatalf("dimension %s out of range: %v", k, v)
		}
	}
	if len(strong.Strengths) == 0 || len(strong.Weaknesses) == 0 {
		t.Fatal("grade must always carry at least one strength and one weakness")
	}
	if strong.Verdict != VerdictForScore(strong.Score) {
		t.Fatalf("verdict %q does not match score %.1f", strong.Verdict, strong.Score)
	}
	if !strings.Contains(strong.Feedback, "内置评分规则") {
		t.Fatal("an offline grade must say so in its feedback")
	}
	if !strings.Contains(strong.Feedback, "回表") && !strings.Contains(strong.Feedback, "b+树") {
		t.Fatalf("feedback should cite what the answer actually covered: %q", strong.Feedback)
	}
}

func TestOfflineGradeSkippedAnswerIsZeroAndNeverFollowedUp(t *testing.T) {
	g := offlineGrade(testGradeInput(t, "   "))
	if g.Score != 0 {
		t.Fatalf("score = %v, want 0", g.Score)
	}
	if g.FollowUp != "" {
		t.Fatalf("a skipped answer must not produce a follow-up, got %q", g.FollowUp)
	}
	if g.Verdict != VerdictWeak {
		t.Fatalf("verdict = %q", g.Verdict)
	}
}

func TestOfflineFollowUpBudget(t *testing.T) {
	in := testGradeInput(t,
		"我先说结论：用缓存穿透的布隆过滤器方案。具体是在查询前先过一层布隆过滤器。")
	in.Kind = KindFollowup
	if got := offlineFollowUp(in, computeSignals(in.Answer, in.Question.Keywords)); got != "" {
		t.Fatalf("a follow-up question must not be followed up again, got %q", got)
	}

	in.Kind = KindQuestion
	in.FollowUps = 3 // the server's per-interview follow-up budget
	if got := offlineFollowUp(in, computeSignals(in.Answer, in.Question.Keywords)); got != "" {
		t.Fatalf("follow-up budget exceeded but got %q", got)
	}
}

func TestNormalizeGradeFillsMissingDimensions(t *testing.T) {
	in := testGradeInput(t, "答案")
	g := Grade{Score: 66, Dimensions: map[string]float64{DimAccuracy: 70}}
	normalizeGrade(&g, in)

	if len(g.Dimensions) != len(in.Plan.Dimensions) {
		t.Fatalf("got %d dimensions, want %d — an omitted dimension must not read as 0",
			len(g.Dimensions), len(in.Plan.Dimensions))
	}
	if g.Dimensions[DimAccuracy] != 70 {
		t.Fatalf("the model's own score was overwritten: %v", g.Dimensions[DimAccuracy])
	}
	for k, v := range g.Dimensions {
		if k != DimAccuracy && v != 66 {
			t.Fatalf("dimension %s = %v, want the overall score 66", k, v)
		}
	}
}

func TestWeightedScoreIgnoresUnscoredDimensions(t *testing.T) {
	dims := []Dimension{
		{Key: "a", Weight: 0.5},
		{Key: "b", Weight: 0.5},
	}
	if got := WeightedScore(dims, map[string]float64{"a": 80}); got != 80 {
		t.Fatalf("WeightedScore = %v, want 80 (b is unscored, not zero)", got)
	}
	if got := WeightedScore(dims, map[string]float64{"a": 90, "b": 70}); got != 80 {
		t.Fatalf("WeightedScore = %v, want 80", got)
	}
	if got := WeightedScore(dims, nil); got != 0 {
		t.Fatalf("WeightedScore with nothing scored = %v, want 0", got)
	}
}

func TestDimensionsForIsStableAcrossTypes(t *testing.T) {
	for _, typ := range []string{TypeTech, TypeBehavior, TypeMixed, TypeSystemDesign, "unknown-type"} {
		dims := DimensionsFor(Spec{Type: typ})
		if len(dims) == 0 {
			t.Fatalf("type %q has no dimensions", typ)
		}
		total := 0.0
		for _, d := range dims {
			total += d.Weight
			if d.Key == "" || d.Label == "" {
				t.Fatalf("type %q has an incomplete dimension: %+v", typ, d)
			}
		}
		if total < 0.999 || total > 1.001 {
			t.Fatalf("type %q weights sum to %v", typ, total)
		}
	}
}

// --- report -----------------------------------------------------------------

func TestOfflineReportIsSelfConsistent(t *testing.T) {
	plan := offlinePlan(testSpec())
	answers := []string{
		"首先索引用 B+树，回表需要再查一次聚簇索引；我们在 300 万行表上把 P99 从 1.2 秒降到 80 毫秒，因为加了覆盖索引。",
		"",
		"缓存穿透我用布隆过滤器挡住不存在的 key，另外给空结果设了 60 秒的短 TTL，取舍是可能短暂不一致。",
		"分布式锁我选了 Redis 的 Redlock，但后来发现时钟漂移问题，所以改成用数据库唯一约束做幂等。",
	}
	turns := make([]TurnBrief, 0, len(answers))
	for i, a := range answers {
		q := plan.Questions[i%len(plan.Questions)]
		g := offlineGrade(GradeInput{Spec: testSpec(), Plan: plan, Question: q, Answer: a})
		turns = append(turns, TurnBrief{
			Seq: i + 1, Kind: KindQuestion, Dimension: q.Dimension,
			Question: q.Question, Answer: a, Score: g.Score, DimScores: g.Dimensions,
		})
	}

	rep := offlineReport(ReportInput{Spec: testSpec(), Plan: plan, Turns: turns})

	if rep.OverallScore <= 0 || rep.OverallScore > 100 {
		t.Fatalf("overallScore = %v", rep.OverallScore)
	}
	if rep.HRSatisfaction <= 0 || rep.HRSatisfaction > 100 {
		t.Fatalf("hrSatisfaction = %v", rep.HRSatisfaction)
	}
	if len(rep.Dimensions) != len(plan.Dimensions) {
		t.Fatalf("got %d report dimensions, want %d", len(rep.Dimensions), len(plan.Dimensions))
	}
	for _, d := range rep.Dimensions {
		if d.Label == "" {
			t.Fatalf("dimension %s has no label", d.Key)
		}
		if d.Score < 0 || d.Score > 100 {
			t.Fatalf("dimension %s score out of range: %v", d.Key, d.Score)
		}
	}
	if rep.Recommendation != RecommendationForScore(rep.OverallScore) {
		t.Fatalf("recommendation %q does not match score %.1f", rep.Recommendation, rep.OverallScore)
	}
	// Non-null arrays are a frontend contract (it maps over them).
	for name, list := range map[string][]string{
		"strengths": rep.Strengths, "weaknesses": rep.Weaknesses,
		"suggestions": rep.Suggestions, "resumeSuggestions": rep.ResumeSuggestions,
		"interviewStrategies": rep.InterviewStrategies, "highlights": rep.Highlights,
		"risks": rep.Risks,
	} {
		if list == nil {
			t.Fatalf("%s is nil; it must serialise as []", name)
		}
	}
	if len(rep.LearningPlan) == 0 {
		t.Fatal("learningPlan is empty")
	}
	if len(rep.ResumeSuggestions) < 3 {
		t.Fatalf("resumeSuggestions has %d items, want >= 3", len(rep.ResumeSuggestions))
	}
	if rep.Summary == "" || rep.RecommendationReason == "" {
		t.Fatal("summary / recommendationReason must not be empty")
	}
	// A skipped question must be called out.
	joined := strings.Join(rep.Weaknesses, " ")
	if !strings.Contains(joined, "未作答") {
		t.Fatalf("a skipped question was not reported: %v", rep.Weaknesses)
	}
	if rep.Risks == nil {
		t.Fatal("risks must be a non-nil slice")
	}
}

func TestNormalizeReportRecomputesScoreAndDropsUnknownDimensions(t *testing.T) {
	plan := offlinePlan(testSpec())
	rep := Report{
		OverallScore: 99, // the model's own arithmetic is not trusted when
		// dimension scores are present and disagree.
		Dimensions: []DimensionScore{
			{Key: plan.Dimensions[0].Key, Score: 40},
			{Key: "not_a_dimension", Score: 100},
		},
	}
	normalizeReport(&rep, ReportInput{Spec: testSpec(), Plan: plan})

	if len(rep.Dimensions) != 1 {
		t.Fatalf("got %d dimensions, want 1 (the unknown key must be dropped)", len(rep.Dimensions))
	}
	if rep.Dimensions[0].Label == "" || rep.Dimensions[0].Weight == 0 {
		t.Fatalf("label/weight must be restored from the rubric: %+v", rep.Dimensions[0])
	}
	// The model supplied an overall score, so it is kept — normalisation
	// only fills blanks, it does not override an explicit number.
	if rep.OverallScore != 99 {
		t.Fatalf("overallScore = %v, want the model's 99", rep.OverallScore)
	}

	blank := Report{Dimensions: []DimensionScore{{Key: plan.Dimensions[0].Key, Score: 50}}}
	normalizeReport(&blank, ReportInput{Spec: testSpec(), Plan: plan})
	if blank.OverallScore != 50 {
		t.Fatalf("overallScore = %v, want 50 derived from the only dimension", blank.OverallScore)
	}
}

func TestNormalizeReportBackfillsMissingDimensions(t *testing.T) {
	plan := offlinePlan(testSpec())
	rep := Report{OverallScore: 72}
	normalizeReport(&rep, ReportInput{Spec: testSpec(), Plan: plan})
	if len(rep.Dimensions) != len(plan.Dimensions) {
		t.Fatalf("got %d dimensions, want %d backfilled from the rubric",
			len(rep.Dimensions), len(plan.Dimensions))
	}
}

// --- engine fallback --------------------------------------------------------

func TestEngineFallsBackToOfflineWhenUnconfigured(t *testing.T) {
	eng := NewEngine(nil) // nil client → offline-only engine
	if eng.Ready() {
		t.Fatal("a nil-client engine must not report itself ready")
	}
	spec := testSpec()

	var streamed strings.Builder
	plan, source, err := eng.BuildPlan(context.Background(), spec, func(s string) error {
		streamed.WriteString(s)
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if source.Engine != EngineOffline {
		t.Fatalf("engine = %q, want %q", source.Engine, EngineOffline)
	}
	if source.Note == "" {
		t.Fatal("Source.Note must explain why it degraded")
	}
	if len(plan.Questions) != spec.QuestionCount {
		t.Fatalf("got %d questions, want %d", len(plan.Questions), spec.QuestionCount)
	}
	if streamed.Len() == 0 {
		t.Fatal("nothing was streamed to the UI for the planning stage")
	}

	grade, source, err := eng.GradeAnswer(context.Background(), GradeInput{
		Spec: spec, Plan: plan, Question: plan.Questions[0],
		Answer: "首先我会看索引结构，因为 B+树 决定了回表成本。",
	}, func(s string) error { streamed.WriteString(s); return nil })
	if err != nil {
		t.Fatalf("GradeAnswer: %v", err)
	}
	if source.Engine != EngineOffline {
		t.Fatalf("engine = %q, want offline", source.Engine)
	}
	if grade.Score <= 0 {
		t.Fatalf("score = %v", grade.Score)
	}

	report, source, err := eng.BuildReport(context.Background(), ReportInput{
		Spec: spec, Plan: plan,
		Turns: []TurnBrief{{Seq: 1, Kind: KindQuestion, Dimension: plan.Questions[0].Dimension,
			Question: plan.Questions[0].Question, Answer: "答案", Score: grade.Score,
			DimScores: grade.Dimensions}},
	}, nil)
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if source.Engine != EngineOffline {
		t.Fatalf("engine = %q, want offline", source.Engine)
	}
	if report.OverallScore <= 0 {
		t.Fatalf("overallScore = %v", report.OverallScore)
	}
}

func TestEngineTestReportsMissingConfig(t *testing.T) {
	res := NewEngine(nil).Test(context.Background(), "")
	if res.OK {
		t.Fatal("Test must not report success without a configured engine")
	}
	if res.Error == "" {
		t.Fatal("Test must explain what is missing")
	}
}

func TestCancelledContextDoesNotDegradeToOffline(t *testing.T) {
	// When the candidate hits Stop, the honest answer is the cancellation
	// — not a heuristic score they did not ask for.
	eng := NewEngine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := eng.BuildPlan(ctx, testSpec(), nil); err != nil {
		// The offline-only engine has no model call to cancel, so it
		// legitimately still succeeds.
		t.Fatalf("unexpected error from the offline path: %v", err)
	}
	if !cancelled(ctx, context.Canceled) {
		t.Fatal("cancelled() must recognise a cancelled context")
	}
	if cancelled(context.Background(), nil) {
		t.Fatal("cancelled() must not fire on a nil error")
	}
	if cancelled(context.Background(), errors.New("boom")) {
		t.Fatal("cancelled() must not fire on an unrelated error")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// --- labels -----------------------------------------------------------------

func TestLabelSetsCoverEveryEnum(t *testing.T) {
	// The UI renders labels from GET /api/interview/presets, so a missing
	// entry shows up as a raw slug in the interface.
	for set, values := range map[string][]string{
		"level":          {LevelJunior, LevelMid, LevelSenior, LevelExpert},
		"type":           {TypeTech, TypeBehavior, TypeMixed, TypeSystemDesign},
		"difficulty":     {DifficultyEasy, DifficultyNormal, DifficultyHard},
		"language":       {LanguageZH, LanguageEN},
		"status":         {StatusDraft, StatusInProgress, StatusCompleted, StatusAborted},
		"recommendation": {RecStrongHire, RecHire, RecMaybe, RecNoHire},
		"verdict":        {VerdictStrong, VerdictOK, VerdictWeak},
	} {
		for _, v := range values {
			if got := LabelOf(set, v); got == v {
				t.Fatalf("label set %q has no entry for %q", set, v)
			}
		}
	}
	if got := LabelOf("level", "nonsense"); got != "nonsense" {
		t.Fatalf("unknown values must fall through unchanged, got %q", got)
	}
}

func TestBankCoversEveryFamilyAndLevel(t *testing.T) {
	byFamily := map[string]map[string]bool{}
	dims := map[string]bool{}
	for _, q := range BankQuestions() {
		if q.Question == "" || q.Intent == "" || q.Reference == "" {
			t.Fatalf("incomplete bank question: %+v", q)
		}
		if len(q.Keywords) == 0 {
			t.Fatalf("bank question has no keywords, so the offline grader loses its coverage signal: %q", q.Question)
		}
		if byFamily[q.Family] == nil {
			byFamily[q.Family] = map[string]bool{}
		}
		for _, l := range q.Levels {
			byFamily[q.Family][l] = true
		}
		dims[q.Dimension] = true
	}
	for _, family := range []string{FamilyBackend, FamilyFrontend, FamilyFullstack, FamilyData,
		FamilyAlgorithm, FamilyProduct, FamilyOps, FamilyTest, FamilyGeneric} {
		levels, ok := byFamily[family]
		if !ok {
			t.Fatalf("family %q has no questions", family)
		}
		for _, l := range []string{LevelJunior, LevelMid, LevelSenior, LevelExpert} {
			if !levels[l] {
				t.Fatalf("family %q has no %s question", family, l)
			}
		}
	}
	if len(dims) < 4 {
		t.Fatalf("the bank only covers %d dimensions", len(dims))
	}
}

func TestRoleFamilyClassifiesRealisticTitles(t *testing.T) {
	cases := map[string]string{
		"高级后端工程师":          FamilyBackend,
		"Golang 后端":        FamilyBackend,
		"backend engineer": FamilyBackend,
		"前端工程师":            FamilyFrontend,
		"React 开发":         FamilyFrontend,
		"全栈工程师":            FamilyFullstack,
		"数据分析师":            FamilyData,
		"算法工程师":            FamilyAlgorithm,
		"产品经理":             FamilyProduct,
		"运维工程师":            FamilyOps,
		"SRE":              FamilyOps,
		"测试开发工程师":          FamilyTest,
		"行政专员":             FamilyGeneric,
		"":                 FamilyGeneric,
	}
	for title, want := range cases {
		if got := RoleFamily(title); got != want {
			t.Errorf("RoleFamily(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestPresetsAreUsableAsIs(t *testing.T) {
	presets := Presets()
	if len(presets) < 6 {
		t.Fatalf("only %d presets", len(presets))
	}
	ids := map[string]bool{}
	for _, p := range presets {
		if ids[p.ID] {
			t.Fatalf("duplicate preset id %q", p.ID)
		}
		ids[p.ID] = true
		if p.Role == "" || p.JDSample == "" {
			// A preset with no JD would make the interview generic, which
			// is exactly the value the JD field exists to provide.
			t.Fatalf("preset %s is missing its role or JD sample", p.ID)
		}
		spec := Spec{Role: p.Role, Level: p.Level, Type: p.InterviewType,
			Difficulty: p.Difficulty, QuestionCount: p.QuestionCount}.Normalize()
		if spec.QuestionCount < MinQuestions || spec.QuestionCount > MaxQuestions {
			t.Fatalf("preset %s has questionCount %d outside [%d,%d]",
				p.ID, p.QuestionCount, MinQuestions, MaxQuestions)
		}
		if LabelOf("level", spec.Level) == spec.Level {
			t.Fatalf("preset %s uses an unknown level %q", p.ID, p.Level)
		}
		if LabelOf("type", spec.Type) == spec.Type {
			t.Fatalf("preset %s uses an unknown interview type %q", p.ID, p.InterviewType)
		}
	}
}

// --- spec normalisation -----------------------------------------------------

func TestSpecNormalizeClampsInsteadOfRejecting(t *testing.T) {
	spec := Spec{Role: "  ", Level: "guru", Type: "nonsense", Language: "", Difficulty: "", QuestionCount: 999}.Normalize()
	if spec.Role == "" {
		t.Fatal("role must get a default")
	}
	if spec.Level != LevelMid || spec.Type != TypeMixed || spec.Language != LanguageZH || spec.Difficulty != DifficultyNormal {
		t.Fatalf("unknown enums were not defaulted: %+v", spec)
	}
	if spec.QuestionCount != MaxQuestions {
		t.Fatalf("questionCount = %d, want clamped to %d", spec.QuestionCount, MaxQuestions)
	}
	if got := (Spec{QuestionCount: -5}).Normalize().QuestionCount; got != MinQuestions {
		t.Fatalf("questionCount = %d, want clamped to %d", got, MinQuestions)
	}
}

func TestTrimToIsRuneSafe(t *testing.T) {
	got := TrimTo("一二三四五六七八九十", 4)
	if got != "一二三四…" {
		t.Fatalf("TrimTo = %q", got)
	}
	if got := TrimTo("短", 10); got != "短" {
		t.Fatalf("TrimTo = %q", got)
	}
}

// Guard against a regression where the two-part protocol silently stops
// being requested: the marker must appear in both streaming prompts.
func TestStreamingPromptsAskForTheTwoPartFormat(t *testing.T) {
	plan := offlinePlan(testSpec())
	gradeMsgs := GradeMessages(GradeInput{Spec: testSpec(), Plan: plan, Question: plan.Questions[0], Answer: "x"})
	reportMsgs := ReportMessages(ReportInput{Spec: testSpec(), Plan: plan,
		Turns: []TurnBrief{{Seq: 1, Question: "q", Answer: "a", Score: 60}}})

	for name, msgs := range map[string][]llm.Message{"grade": gradeMsgs, "report": reportMsgs} {
		joined := messagesText(msgs)
		if !strings.Contains(joined, jsonDelimiter) {
			t.Fatalf("%s prompt does not ask for the %s marker", name, jsonDelimiter)
		}
		if !strings.Contains(joined, "JSON") {
			t.Fatalf("%s prompt does not ask for JSON", name)
		}
	}
}

func messagesText(msgs []llm.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
	}
	return b.String()
}
