package interview

import (
	"fmt"
	"regexp"
	"strings"
)

// offline.go is the deterministic fallback: a rubric-driven planner,
// grader and report writer built on the bundled question bank.
//
// Why it exists rather than a "model unavailable" error page:
//
//   - a fresh install with no API key still demonstrates the whole
//     product, including a scored report
//   - it is what the unit tests assert against, so the domain logic has
//     coverage that does not depend on a network call
//   - when a configured model fails mid-interview the session continues
//     instead of dead-ending, and Source tells the user it degraded
//
// It is honestly labelled everywhere in the UI: Source.Engine == "offline"
// and Source.Note explains why.
//
// Grading is signal-based. Each signal is a 0..1 estimate of one
// observable property of the answer text, and each dimension combines
// the signals that are relevant to it:
//
//	coverage  fraction of the question's expected keywords present
//	length    how far the answer gets toward a substantive reply
//	structure enumeration/paragraphing — is it organised?
//	detail    numbers, metrics, versions, named technologies
//	reasoning causal language — does it explain WHY, not just WHAT?
//	evidence  first-person project experience — does it ground in reality?
//
// Signals that cannot be measured (a keyword-less question produced by
// the LLM path) carry weight 0 and are redistributed, so a missing signal
// never silently counts as a failure.
type signals struct {
	coverage     float64
	hasKeywords  bool
	length       float64
	structure    float64
	detail       float64
	reasoning    float64
	evidence     float64
	missingTerms []string
	hitTerms     []string
	runes        int
}

// weighted is one signal reading paired with how much it counts toward a
// dimension. A reading whose weight is 0 is simply not measurable here
// (e.g. keyword coverage for an LLM-authored question, which carries no
// keywords) and drops out of the average instead of scoring zero.
type weighted struct {
	v float64
	w float64
}

// mix combines weighted signals into a 0..100 score, renormalising over
// whatever weight is actually available.
func mix(sigs ...weighted) float64 {
	total, weight := 0.0, 0.0
	for _, s := range sigs {
		if s.w <= 0 {
			continue
		}
		total += s.v * s.w
		weight += s.w
	}
	if weight == 0 {
		return 0
	}
	return clamp(100*total/weight, 0, 100)
}

func sig(v, w float64) weighted { return weighted{v: v, w: w} }

// --- text signals -----------------------------------------------------------

var (
	// TL;DR: these patterns are proxies for "the answer shows its work".
	structureRe = regexp.MustCompile(`(?m)(^|\n)\s*(\d+[.、)]|[-*•]|首先|其次|然后|接着|最后|第一|第二|第三|一方面|另一方面|另外)`)
	reasoningRe = regexp.MustCompile(`因为|所以|因此|导致|原因是|根本上|本质|权衡|取舍|取舍|瓶颈|为什么|取决于|前提|假设|复杂度|trade-?off|越.*越`)
	detailRe    = regexp.MustCompile(`\d+(\.\d+)?\s*(ms|毫秒|s\b|秒|%|％|倍|万|亿|qps|QPS|tps|TPS|GB|MB|TB|核|人|天|周|月|年|次|条|台|个)|` +
		`\b(v?\d+(\.\d+){1,2})\b|` +
		`\b(SQL|HTTP|HTTPS|TCP|UDP|Redis|MySQL|Postgres|Kafka|Docker|K8s|Kubernetes|JVM|GC|CPU|IO|SSD|CDN|API|RPC|gRPC|OLAP|OLTP|RAG|LLM|SSE|WebSocket|React|Vue|Go|Golang|Java|Python|Rust|TypeScript|ES|Elasticsearch)\b`)
	evidenceRe = regexp.MustCompile(`我(在|们|负责|做过|实现|主导|参与|设计|遇到|踩过|排查)|我们(团队|项目|系统|线上|公司)|项目里|线上|实战|生产环境|工作中|之前(的|在)`)
)

func computeSignals(answer string, keywords []string) signals {
	text := strings.TrimSpace(answer)
	var s signals
	s.runes = len([]rune(text))
	if s.runes == 0 {
		return s
	}
	lower := strings.ToLower(text)

	// Coverage: keywords are lowercase tokens (possibly Chinese).
	if len(keywords) > 0 {
		s.hasKeywords = true
		hit := 0
		for _, k := range keywords {
			k = strings.ToLower(strings.TrimSpace(k))
			if k == "" {
				continue
			}
			if strings.Contains(lower, k) {
				hit++
				s.hitTerms = append(s.hitTerms, k)
			} else {
				s.missingTerms = append(s.missingTerms, k)
			}
		}
		if total := hit + len(s.missingTerms); total > 0 {
			s.coverage = float64(hit) / float64(total)
		}
	}

	s.length = clamp(float64(s.runes)/260, 0, 1)
	s.structure = clamp(float64(len(structureRe.FindAllString(text, -1)))/3, 0, 1)
	s.detail = clamp(float64(len(detailRe.FindAllString(text, -1)))/3, 0, 1)
	s.reasoning = clamp(float64(len(reasoningRe.FindAllString(text, -1)))/2, 0, 1)
	s.evidence = clamp(float64(len(evidenceRe.FindAllString(text, -1)))/1.5, 0, 1)
	return s
}

// scoreForDimension maps the signals onto one rubric dimension.
func (s signals) scoreForDimension(key string) float64 {
	cov := sig(s.coverage, 0)
	if s.hasKeywords {
		cov.w = 0.55
	}
	switch key {
	case DimAccuracy:
		return mix(cov, sig(s.detail, 0.25), sig(s.length, 0.20))
	case DimDepth:
		return mix(cov, sig(s.reasoning, 0.30), sig(s.length, 0.25))
	case DimProblemSolving:
		return mix(cov, sig(s.reasoning, 0.30), sig(s.structure, 0.30))
	case DimCommunication:
		return mix(sig(s.structure, 0.50), sig(s.length, 0.30), sig(s.evidence, 0.20))
	case DimSituational:
		return mix(sig(s.evidence, 0.35), sig(s.structure, 0.35), sig(s.length, 0.30))
	case DimTeamwork:
		return mix(sig(s.evidence, 0.40), sig(s.structure, 0.30), sig(s.length, 0.30))
	case DimSelfAwareness:
		return mix(sig(s.length, 0.40), sig(s.reasoning, 0.30), sig(s.structure, 0.30))
	case DimLogic:
		return mix(sig(s.structure, 0.45), sig(s.reasoning, 0.35), sig(s.length, 0.20))
	case DimFit:
		return mix(sig(s.evidence, 0.40), cov, sig(s.length, 0.30))
	default: // DimProfessional
		return mix(cov, sig(s.detail, 0.30), sig(s.length, 0.20))
	}
}

// --- planner ----------------------------------------------------------------

// dimAffinity maps a bank question's own dimension onto the rubric
// actually in force for this interview, in order of preference.
//
// This exists because the bank is organised by *domain* (a backend
// question probes 技术准确性/技术深度) while the rubric is organised by
// *interview type* — 综合面试 scores 专业能力/逻辑思维/沟通表达/岗位匹配
// instead. Without this mapping, every bank question whose dimension is
// not literally in the rubric is unreachable and a 综合面试 paper comes
// out half-empty. The first entry present in the rubric wins.
var dimAffinity = map[string][]string{
	DimAccuracy:       {DimProfessional, DimAccuracy, DimLogic},
	DimDepth:          {DimDepth, DimProfessional, DimProblemSolving},
	DimProblemSolving: {DimProblemSolving, DimLogic, DimProfessional},
	DimCommunication:  {DimCommunication, DimFit},
	DimSituational:    {DimFit, DimSituational, DimProblemSolving},
	DimTeamwork:       {DimFit, DimTeamwork, DimCommunication},
	DimSelfAwareness:  {DimFit, DimSelfAwareness, DimCommunication},
	DimProfessional:   {DimProfessional, DimAccuracy, DimDepth},
	DimLogic:          {DimLogic, DimProblemSolving, DimProfessional},
	DimFit:            {DimFit, DimCommunication, DimProfessional},
}

// rubricDimensionFor resolves the rubric key a bank question is SCORED
// under. An exact match always wins.
func rubricDimensionFor(bankDim string, dims []Dimension) string {
	if _, ok := DimensionByKey(dims, bankDim); ok {
		return bankDim
	}
	for _, candidate := range dimAffinity[bankDim] {
		if _, ok := DimensionByKey(dims, candidate); ok {
			return candidate
		}
	}
	if len(dims) > 0 {
		return dims[0].Key
	}
	return bankDim
}

// offlinePlan assembles a paper from the bundled bank, round-robining
// across the rubric dimensions so every axis is exercised.
func offlinePlan(spec Spec) Plan {
	spec = spec.Normalize()
	dims := NormalizeDimensions(DimensionsFor(spec))
	family := RoleFamily(spec.Role)
	pool := bankPool(family, spec.Level)

	// Bucket by the dimension each question will be scored under, not the
	// dimension it was authored with — see dimAffinity.
	byDim := map[string][]BankQuestion{}
	for _, q := range pool {
		key := rubricDimensionFor(q.Dimension, dims)
		q.Dimension = key
		byDim[key] = append(byDim[key], q)
	}
	cursor := map[string]int{}
	picked := make([]BankQuestion, 0, spec.QuestionCount)
	for len(picked) < spec.QuestionCount {
		progressed := false
		for _, d := range dims {
			if len(picked) >= spec.QuestionCount {
				break
			}
			list := byDim[d.Key]
			if cursor[d.Key] >= len(list) {
				continue
			}
			picked = append(picked, list[cursor[d.Key]])
			cursor[d.Key]++
			progressed = true
		}
		if !progressed {
			break
		}
	}
	if len(picked) == 0 {
		// The bank is compiled in, so this is unreachable in practice —
		// but a zero-question interview would be a much worse failure
		// than a generic one.
		picked = emergencyQuestions(spec)
	}

	questions := make([]PlannedQuestion, 0, len(picked))
	for i, q := range picked {
		questions = append(questions, PlannedQuestion{
			Seq:       i + 1,
			Dimension: q.Dimension,
			Question:  q.Question,
			Intent:    q.Intent,
			Weight:    1,
			Reference: q.Reference,
			Keywords:  q.Keywords,
		})
	}

	return Plan{
		Dimensions: dims,
		Questions:  questions,
		Rationale: fmt.Sprintf(
			"本场未调用大模型：使用内置题库按「%s · %s」自动组卷，共 %d 题，覆盖 %s。评分采用内置规则（关键词覆盖率、表达结构、技术细节、因果推理、实战佐证五个信号）。",
			spec.Role, LabelOf("level", spec.Level), len(questions), dimensionLabels(dims)),
	}
}

// bankPool prefers level-matched questions from the right family, then
// level-matched generic ones, then any question from the family. Order is
// stable, so the same spec always yields the same paper.
func bankPool(family, level string) []BankQuestion {
	all := BankQuestions()
	var exact, generic, familyAny []BankQuestion
	for _, q := range all {
		switch {
		case q.Family == family && containsStr(q.Levels, level):
			exact = append(exact, q)
		case q.Family == family:
			familyAny = append(familyAny, q)
		case q.Family == FamilyGeneric && containsStr(q.Levels, level):
			generic = append(generic, q)
		}
	}
	out := make([]BankQuestion, 0, len(exact)+len(generic)+len(familyAny))
	out = append(out, exact...)
	out = append(out, generic...)
	out = append(out, familyAny...)
	return out
}

func dimensionLabels(dims []Dimension) string {
	labels := make([]string, 0, len(dims))
	for _, d := range dims {
		labels = append(labels, d.Label)
	}
	return strings.Join(labels, "、")
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// emergencyQuestions is the last line of defence: used only if the bank
// somehow yields nothing for a role.
func emergencyQuestions(spec Spec) []BankQuestion {
	level := spec.Level
	return []BankQuestion{
		{Family: FamilyGeneric, Dimension: DimProfessional, Levels: []string{level},
			Question:  "请挑一个你最近一年投入最多的项目，说明你的具体职责、技术选型理由和最终结果。",
			Intent:    "考察真实项目经历与技术决策能力",
			Reference: "能说清背景、约束、备选方案、选择理由和可量化的结果。",
			Keywords:  []string{"背景", "职责", "选型", "原因", "结果", "指标"}},
		{Family: FamilyGeneric, Dimension: DimProblemSolving, Levels: []string{level},
			Question:  "讲一次你排查线上问题的经历：现象是什么，你怎么定位，最后根因是什么？",
			Intent:    "考察问题定位方法与工程判断",
			Reference: "有明确的排查路径（日志/监控/复现/二分），能区分现象与根因，并给出防复发措施。",
			Keywords:  []string{"现象", "日志", "监控", "复现", "根因", "修复", "复盘"}},
		{Family: FamilyGeneric, Dimension: DimDepth, Levels: []string{level},
			Question:  "选一个你最熟悉的技术，讲讲它的实现原理以及它的边界条件。",
			Intent:    "考察原理级理解与边界意识",
			Reference: "能讲到机制层面，并指出失效场景与替代方案的取舍。",
			Keywords:  []string{"原理", "机制", "边界", "失效", "取舍", "替代"}},
		{Family: FamilyGeneric, Dimension: DimCommunication, Levels: []string{level},
			Question:  "如果要把一个复杂技术方案讲给非技术的同事听，你会怎么组织内容？",
			Intent:    "考察结构化表达与换位思考",
			Reference: "先结论后细节，用类比和业务语言，明确对方需要做的决策。",
			Keywords:  []string{"结论", "类比", "重点", "业务", "决策", "受众"}},
		{Family: FamilyGeneric, Dimension: DimLogic, Levels: []string{level},
			Question:  "请描述一个你做过的权衡（性能 vs 成本、速度 vs 质量），你的判断依据是什么？",
			Intent:    "考察推理链条与取舍标准",
			Reference: "明确约束与优先级，给出量化依据，并说明放弃了什么。",
			Keywords:  []string{"权衡", "约束", "优先级", "依据", "成本", "收益"}},
		{Family: FamilyGeneric, Dimension: DimFit, Levels: []string{level},
			Question:  fmt.Sprintf("你为什么认为自己适合「%s」这个岗位？请结合具体经历说明。", spec.Role),
			Intent:    "考察岗位匹配度与自我定位",
			Reference: "把岗位要求逐条对应到自身经历，坦诚说明差距与补齐计划。",
			Keywords:  []string{"岗位", "匹配", "经历", "优势", "差距", "计划"}},
	}
}

// --- grader -----------------------------------------------------------------

func offlineGrade(in GradeInput) Grade {
	dims := in.Plan.Dimensions
	if len(dims) == 0 {
		dims = NormalizeDimensions(DimensionsFor(in.Spec))
	}
	answer := strings.TrimSpace(in.Answer)

	if answer == "" {
		return zeroGrade(dims, in.Question.Reference,
			"本题没有作答记录。跳过会直接按 0 分计入总评，"+
				"如果是时间不够，下次可以先给出思路框架再补细节——面试官通常会给框架分。",
			"本题未作答，无法评估")
	}

	s := computeSignals(answer, in.Question.Keywords)

	// Substance gate: a reply with no keyword coverage and no technical
	// detail, and barely any text, carries nothing to assess.
	//
	// TrivialAnswer is checked separately and first because the signal
	// regexes defeat the weaker test: "Go" and "SQL" are 2-3 runes, yet
	// detailRe matches them as named technologies, so the answer would earn
	// a non-zero score and a generic critique while the accompanying note
	// said "too short to assess". Same rule in both paths, or the two
	// disagree about the same answer.
	if TrivialAnswer(answer) || (s.runes < 15 && s.coverage == 0 && s.detail == 0) {
		return zeroGrade(dims, in.Question.Reference,
			"这个回答过于简短，无法构成一次有效作答。"+
				"面试里可以用\"我了解的部分是…，这块我还不确定\"来给出信息量，这比一句"+
				"\"不太清楚\"或几个字母能拿到的分数高得多。",
			"回答过于简短，没有可评估的内容")
	}

	scores := make(map[string]float64, len(dims))
	for _, d := range dims {
		scores[d.Key] = round1(s.scoreForDimension(d.Key))
	}
	base := WeightedScore(dims, scores)

	// Difficulty shifts the bar, not the answer: the same reply is worth
	// more in an easy interview than in a hard one.
	multiplier := 1.0
	switch in.Spec.Difficulty {
	case DifficultyEasy:
		multiplier = 1.06
	case DifficultyHard:
		multiplier = 0.94
	}
	final := round1(clamp(base*multiplier, 0, 100))
	for k := range scores {
		scores[k] = round1(clamp(scores[k]*multiplier, 0, 100))
	}

	strengths, weaknesses := narrateSignals(s)
	return Grade{
		Score:      final,
		Dimensions: scores,
		Strengths:  strengths,
		Weaknesses: weaknesses,
		Feedback:   offlineFeedback(s, final, weaknesses, in.Question.Keywords),
		Reference:  in.Question.Reference,
		FollowUp:   offlineFollowUp(in, s),
		Verdict:    VerdictForScore(final),
	}
}

// zeroGrade is the "nothing to assess" outcome, shared by the empty and
// non-substantive cases so they cannot drift apart in shape.
func zeroGrade(dims []Dimension, reference, feedback, weakness string) Grade {
	zero := make(map[string]float64, len(dims))
	for _, d := range dims {
		zero[d.Key] = 0
	}
	return Grade{
		Score:      0,
		Dimensions: zero,
		Strengths:  []string{},
		Weaknesses: []string{weakness},
		Feedback:   feedback,
		Reference:  reference,
		FollowUp:   "",
		Verdict:    VerdictWeak,
	}
}

func narrateSignals(s signals) (strengths, weaknesses []string) {
	if s.hasKeywords && s.coverage >= 0.6 {
		strengths = append(strengths, fmt.Sprintf("覆盖了本题主要考点（%s）", strings.Join(topN(s.hitTerms, 4), "、")))
	}
	if s.detail >= 0.34 {
		strengths = append(strengths, "给出了具体的技术细节或量化数据，可信度较高")
	}
	if s.reasoning >= 0.5 {
		strengths = append(strengths, "解释了背后的原因与取舍，不止停留在\"怎么做\"")
	}
	if s.structure >= 0.34 {
		strengths = append(strengths, "表达有结构，能分层展开")
	}
	if s.evidence >= 0.34 {
		strengths = append(strengths, "能结合自身项目经历作答")
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "给出了正面回应，态度明确")
	}

	if s.runes < 80 {
		weaknesses = append(weaknesses, fmt.Sprintf("回答偏短（%d 字），关键点没有展开", s.runes))
	}
	if s.hasKeywords && s.coverage < 0.5 && len(s.missingTerms) > 0 {
		weaknesses = append(weaknesses, fmt.Sprintf("缺少本题关键要点：%s", strings.Join(topN(s.missingTerms, 4), "、")))
	}
	if s.reasoning < 0.34 {
		weaknesses = append(weaknesses, "只描述了做法，没有说明原因、代价与适用边界")
	}
	if s.detail < 0.2 {
		weaknesses = append(weaknesses, "缺少量化结果或具体技术名词，说服力不足")
	}
	if s.structure < 0.2 && s.runes >= 80 {
		weaknesses = append(weaknesses, "内容较散，建议按\"结论—理由—例子\"组织")
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "整体成立，但在深度上还可以再挖一层")
	}
	return strengths, weaknesses
}

func offlineFeedback(s signals, score float64, weaknesses []string, keywords []string) string {
	var b strings.Builder
	switch {
	case score >= 80:
		b.WriteString("这是一个**扎实的回答**。")
	case score >= 60:
		b.WriteString("这个回答**基本成立**，但离\"有说服力\"还有距离。")
	default:
		b.WriteString("这个回答**没有达到本岗位的期待**，需要重点补强。")
	}
	if s.hasKeywords && s.coverage >= 0.5 {
		fmt.Fprintf(&b, "你提到了 %s 等关键点，方向是对的。", strings.Join(topN(s.hitTerms, 4), "、"))
	}
	if s.reasoning >= 0.5 {
		b.WriteString("而且你解释了原因与取舍，这是加分项。")
	}
	b.WriteString("\n\n**主要问题**\n")
	for _, w := range weaknesses {
		fmt.Fprintf(&b, "- %s\n", w)
	}
	if len(keywords) > 0 {
		fmt.Fprintf(&b, "\n> 本题参考要点：%s", strings.Join(topN(keywords, 8), "、"))
	}
	b.WriteString("\n\n（本点评由内置评分规则生成：未配置可用的模型服务，因此按关键词覆盖、表达结构、技术细节、因果推理、实战佐证五个信号打分，不含语义理解。）")
	return b.String()
}

// offlineFollowUp digs one level deeper only when the answer is
// promising-but-incomplete. A weak answer is not worth a follow-up: the
// candidate does not know the material, and pressing costs everyone time.
func offlineFollowUp(in GradeInput, s signals) string {
	if in.Kind == KindFollowup || in.FollowUps >= 3 {
		return ""
	}
	answer := strings.TrimSpace(in.Answer)
	if len([]rune(answer)) < 60 || s.coverage >= 0.85 {
		return ""
	}
	if len(s.missingTerms) > 0 {
		return fmt.Sprintf("你刚才没有展开 %s 这部分——在你的实际项目里，这一块具体是怎么做的？", s.missingTerms[0])
	}
	if len(s.hitTerms) > 0 {
		return fmt.Sprintf("你提到 %s，能具体说一下它在你们系统里的实际落地方式和踩过的坑吗？", s.hitTerms[0])
	}
	return ""
}

func topN(list []string, n int) []string {
	out := uniqStrings(list)
	if len(out) > n {
		out = out[:n]
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// --- report -----------------------------------------------------------------

func offlineReport(in ReportInput) Report {
	dims := in.Plan.Dimensions
	if len(dims) == 0 {
		dims = NormalizeDimensions(DimensionsFor(in.Spec))
	}

	// Average each dimension across the turns that were scored on it.
	sums := map[string]float64{}
	counts := map[string]int{}
	overallSum, scored, skipped := 0.0, 0, 0
	for _, t := range in.Turns {
		if strings.TrimSpace(t.Answer) == "" {
			skipped++
		} else {
			scored++
		}
		overallSum += t.Score
		for k, v := range t.DimScores {
			sums[k] += v
			counts[k]++
		}
	}
	avg := 0.0
	if len(in.Turns) > 0 {
		avg = overallSum / float64(len(in.Turns))
	}

	dimScores := make(map[string]float64, len(dims))
	for _, d := range dims {
		if counts[d.Key] > 0 {
			dimScores[d.Key] = round1(sums[d.Key] / float64(counts[d.Key]))
		} else {
			dimScores[d.Key] = round1(avg)
		}
	}
	overall := WeightedScore(dims, dimScores)

	// HR satisfaction is not the technical score: screening is also about
	// how clearly the candidate presents. Weight the communication axis
	// in explicitly so the two numbers can differ for a reason.
	comm := dimScores[DimCommunication]
	if comm == 0 {
		comm = overall
	}
	hr := round1(clamp(0.78*overall+0.22*comm, 0, 100))

	ranked := append([]Dimension(nil), dims...)
	sortDimsByScore(ranked, dimScores)

	strengths := []string{}
	weaknesses := []string{}
	suggestions := []string{}
	for i, d := range ranked {
		sc := dimScores[d.Key]
		switch {
		case i < 2 && sc >= 70:
			strengths = append(strengths,
				fmt.Sprintf("%s（%.0f 分）是本场相对优势：%s", d.Label, sc, dimensionPraise(d.Key, sc)))
		case i >= len(ranked)-2 && sc < 70:
			weaknesses = append(weaknesses,
				fmt.Sprintf("%s（%.0f 分）是主要短板：%s", d.Label, sc, dimensionCritique(d.Key, sc)))
			suggestions = append(suggestions, dimensionAdvice(d.Key))
		}
	}
	if len(strengths) == 0 {
		strengths = append(strengths, fmt.Sprintf("完成了 %d 道题的作答，整体表现稳定在 %.0f 分附近", len(in.Turns), overall))
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "各维度都在合格线以上，差距主要在于缺少能把分数拉开的高光回答")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "在下一场面试中刻意练习\"结论先行\"：先用一句话给答案，再展开理由与例子")
	}
	if skipped > 0 {
		weaknesses = append(weaknesses, fmt.Sprintf("有 %d 道题未作答，直接按 0 分计入总评，会明显拉低整体分数", skipped))
		suggestions = append(suggestions, "遇到不会的题也要给出解题框架："+
			"先复述问题、说明你的判断路径，再给出你能确定的部分，通常能拿到框架分")
	}

	var highlights []string
	best, bestScore := "", 0.0
	for _, t := range in.Turns {
		if t.Score > bestScore && strings.TrimSpace(t.Answer) != "" {
			best, bestScore = t.Answer, t.Score
		}
	}
	if best != "" {
		highlights = append(highlights, TrimTo(best, 160))
	}

	risks := []string{}
	lowCount := 0
	for _, t := range in.Turns {
		if t.Score < 55 && strings.TrimSpace(t.Answer) != "" {
			lowCount++
		}
	}
	switch {
	case skipped >= 2:
		risks = append(risks, "多题未作答，无法判断真实水平，面试官可能直接判定为准备不足")
	case lowCount >= 2:
		risks = append(risks, fmt.Sprintf("有 %d 道题得分低于 55 分，集中在核心考点上，是简历难以掩盖的硬伤", lowCount))
	case immediatelyEmpty(in.Turns):
		risks = append(risks, "全部题目均无有效作答，本次报告仅反映作答完整度")
	}
	if in.Spec.Resume == "" {
		risks = append(risks, "本次未提供简历，题目与评估未结合你的真实经历，结论的针对性有限")
	}
	if in.Spec.JD == "" {
		risks = append(risks, "本次未提供岗位描述，评估基于岗位名称的通用要求")
	}

	resumeSuggestions := resumeAdvice(in.Spec)
	strategies := interviewStrategies(dimScores, dims)

	report := Report{
		OverallScore:        overall,
		HRSatisfaction:      hr,
		Summary:             offlineSummary(in, overall, hr, ranked, dimScores, scored, skipped),
		Strengths:           strengths,
		Weaknesses:          weaknesses,
		Suggestions:         suggestions,
		ResumeSuggestions:   resumeSuggestions,
		InterviewStrategies: strategies,
		LearningPlan:        learningPlan(ranked, dimScores),
		Recommendation:      RecommendationForScore(overall),
		Highlights:          highlights,
		Risks:               risks,
	}
	for _, d := range dims {
		report.Dimensions = append(report.Dimensions, DimensionScore{
			Key: d.Key, Label: d.Label, Weight: d.Weight,
			Score:   dimScores[d.Key],
			Comment: dimensionComment(d.Key, dimScores[d.Key]),
		})
	}
	report.RecommendationReason = recommendationReason(overall, ranked, dimScores)
	report.Normalize(dims)
	return report
}

func immediatelyEmpty(turns []TurnBrief) bool {
	for _, t := range turns {
		if strings.TrimSpace(t.Answer) != "" {
			return false
		}
	}
	return len(turns) > 0
}

func offlineSummary(in ReportInput, overall, hr float64, ranked []Dimension, scores map[string]float64, scored, skipped int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## 总体结论\n\n")
	fmt.Fprintf(&b, "本场「%s · %s」模拟面试综合得分 **%.1f 分**，预测 HR 满意度 **%.0f%%**，结论：**%s**。\n\n",
		in.Spec.Role, LabelOf("level", in.Spec.Level), overall, hr, LabelOf("recommendation", RecommendationForScore(overall)))

	if len(ranked) >= 2 {
		fmt.Fprintf(&b, "表现最好的是 **%s（%.0f 分）**，最需要补强的是 **%s（%.0f 分）**。\n\n",
			ranked[0].Label, scores[ranked[0].Key],
			ranked[len(ranked)-1].Label, scores[ranked[len(ranked)-1].Key])
	}
	fmt.Fprintf(&b, "本场共 %d 题，有效作答 %d 题", len(in.Turns), scored)
	if skipped > 0 {
		fmt.Fprintf(&b, "，未作答 %d 题", skipped)
	}
	b.WriteString("。\n\n")

	b.WriteString("## 下一步\n\n")
	b.WriteString("1. 先按下面的「简历改进建议」改一版简历，把经历写成本场答题时那种\"有背景、有取舍、有结果\"的结构。\n")
	fmt.Fprintf(&b, "2. 针对 %s 做一次专项复习，然后用同样的岗位再跑一场面试，对比两次的维度分。\n",
		ranked[len(ranked)-1].Label)
	b.WriteString("3. 面试时刻意使用「结论 → 理由 → 例子 → 结果」的顺序，这是本场失分最集中的地方。\n\n")

	b.WriteString("> 说明：本报告由内置评分规则生成（未配置可用模型服务），分数基于关键词覆盖率、表达结构、技术细节、因果推理与实战佐证五个可观测信号，不含语义理解，请作为自查清单而非精确评价。")
	return b.String()
}

func sortDimsByScore(dims []Dimension, scores map[string]float64) {
	// Insertion sort: dimension sets are 4 items, and this keeps the
	// ordering stable for equal scores without importing sort.
	for i := 1; i < len(dims); i++ {
		for j := i; j > 0 && scores[dims[j].Key] > scores[dims[j-1].Key]; j-- {
			dims[j], dims[j-1] = dims[j-1], dims[j]
		}
	}
}

func dimensionPraise(key string, score float64) string {
	switch key {
	case DimAccuracy:
		return "事实与技术概念基本准确，没有明显硬伤"
	case DimDepth:
		return "能讲到原理层面，而不是只停留在 API 用法"
	case DimProblemSolving:
		return "有清晰的排查/分析路径，能收敛到可执行结论"
	case DimCommunication:
		return "表达有条理，重点容易抓住"
	case DimSituational:
		return "对真实场景的判断和取舍站得住"
	case DimTeamwork:
		return "能讲清协作方式与推动过程"
	case DimSelfAwareness:
		return "对自身短板有清醒认识"
	case DimLogic:
		return "推理链条完整，结论有依据"
	case DimFit:
		return "经历与岗位要求对应得上"
	default:
		return "专业基本功扎实"
	}
}

func dimensionCritique(key string, score float64) string {
	switch key {
	case DimAccuracy:
		return "存在概念混淆或事实性偏差，面试官会据此怀疑你的技术底子"
	case DimDepth:
		return "回答停留在\"怎么做\"，没有解释\"为什么\"和边界条件"
	case DimProblemSolving:
		return "缺少结构化的分析路径，容易给人\"靠感觉试\"的印象"
	case DimCommunication:
		return "内容散、重点不突出，面试官需要自己提炼你的结论"
	case DimSituational:
		return "回答偏理论，缺少真实场景的细节支撑"
	case DimTeamwork:
		return "更多在讲个人产出，协作与推动过程讲得少"
	case DimSelfAwareness:
		return "对自身能力边界描述模糊"
	case DimLogic:
		return "结论与理由之间缺少连接，说服力不足"
	case DimFit:
		return "对岗位要求的理解不够具体，匹配度没有讲透"
	default:
		return "专业细节不够扎实"
	}
}

func dimensionAdvice(key string) string {
	switch key {
	case DimAccuracy:
		return "把本岗位最核心的 5 个概念做成卡片，能用自己的话讲清\"是什么、为什么、什么场景不适用\""
	case DimDepth:
		return "针对每道题追问自己三次\"为什么\"，直到讲不下去为止，那里就是你的知识边界"
	case DimProblemSolving:
		return "用 STAR + 排查树的方式重写你的三个项目故事：现象 → 假设 → 验证 → 根因 → 防复发"
	case DimCommunication:
		return "练习\"结论先行\"：每个回答先给一句结论，再给不超过三个支撑点，最后给一个具体例子"
	case DimSituational:
		return "为每个常见情境准备一个真实案例（冲突、延期、需求变更、线上事故）"
	case DimTeamwork:
		return "把项目描述从\"我做了什么\"改成\"我们遇到什么阻力、我如何推动\""
	case DimSelfAwareness:
		return "准备一段坦诚的能力评估：擅长什么、正在补什么、用什么证据证明"
	case DimLogic:
		return "回答时显式使用连接词（因为/所以/但是/因此），让面试官听得出你的推理链"
	case DimFit:
		return "把岗位 JD 逐条拆成要求，每条后面写上自己对应的经历或补齐计划"
	default:
		return "系统性复习岗位核心知识，并做至少两轮复盘式练习"
	}
}

func dimensionComment(key string, score float64) string {
	switch {
	case score >= 85:
		return "表现优秀：" + dimensionPraise(key, score)
	case score >= 70:
		return "表现良好：" + dimensionPraise(key, score)
	case score >= 60:
		return "基本合格，但有明显提升空间：" + dimensionCritique(key, score)
	default:
		return "不达预期：" + dimensionCritique(key, score)
	}
}

func recommendationReason(overall float64, ranked []Dimension, scores map[string]float64) string {
	weakest := ranked[len(ranked)-1]
	switch RecommendationForScore(overall) {
	case RecStrongHire:
		return fmt.Sprintf("综合 %.1f 分，各维度普遍在 85 分以上，可以直接进入下一轮。", overall)
	case RecHire:
		return fmt.Sprintf("综合 %.1f 分，能力满足岗位要求；%s（%.0f 分）偏低，建议下一轮重点确认。",
			overall, weakest.Label, scores[weakest.Key])
	case RecMaybe:
		return fmt.Sprintf("综合 %.1f 分，处于临界区间，%s（%.0f 分）明显偏弱，需要更多证据才能判断。",
			overall, weakest.Label, scores[weakest.Key])
	default:
		return fmt.Sprintf("综合 %.1f 分，%s（%.0f 分）等核心维度不达岗位基准，建议先补齐再投同类岗位。",
			overall, weakest.Label, scores[weakest.Key])
	}
}

func resumeAdvice(spec Spec) []string {
	out := []string{
		"把项目经历改写成「背景 → 你的职责 → 技术选型与取舍 → 可量化结果」四段式，每条不超过三行，删掉\"参与/负责/协助\"这类无信息量的词。",
		"为每个项目补上至少一个量化结果（QPS 提升、耗时下降、成本节约、覆盖用户数），没有精确数据就写数量级区间，例如\"日均请求量 10 万级\"。",
		"把技能栏按「熟练 / 熟悉 / 了解」三档分开，并只保留能在面试中讲清原理的技术——面试官会挑你最靠前的技术问。",
	}
	switch spec.Level {
	case LevelJunior:
		out = append(out, "初级岗位更看重潜力与基础：把课程设计、开源贡献、实习经历放到最显眼的位置，并写清你具体实现了哪一部分。")
	case LevelSenior, LevelExpert:
		out = append(out, "高级/资深岗位要突出影响面：写清你主导的方案影响了多少团队/系统、解决了什么长期问题，而不只是用了哪些技术。")
	default:
		out = append(out, "中级岗位要在\"能独立交付\"上给证据：挑两个你从设计到上线全程负责的模块重点写。")
	}
	if spec.JD != "" {
		out = append(out, "对照 JD 逐条核对：JD 出现的每个关键词（技术栈、业务领域、能力要求）都应该在简历里有明确对应，没有的要么补上经历、要么删掉不相关的内容腾出篇幅。")
	} else {
		out = append(out, "补一份目标岗位的 JD 再跑一次面试：本场没有 JD，简历与岗位的匹配度无法针对性评估。")
	}
	out = append(out, "压缩非技术内容（兴趣爱好、自我评价占一半篇幅是常见问题），把篇幅让给最能证明你能力的两个项目。")
	return out
}

func interviewStrategies(scores map[string]float64, dims []Dimension) []string {
	out := []string{
		"每题都用「结论先行」开场：先用一句话给出答案，再展开理由和例子——面试官最怕的是听完三分钟还不知道你的观点。",
		"回答里主动带上取舍：说明你放弃了什么、为什么，这比只讲方案更能体现工程判断力。",
		"遇到不会的题不要沉默或硬编：复述问题确认理解、说出你的判断路径、给出你能确定的部分，通常能拿到框架分。",
	}
	if scores[DimDepth] < 70 {
		out = append(out, "准备\"三层深挖\"话术：对每个你写在简历上的技术，都能回答\"原理是什么、边界在哪、出过什么故障\"。")
	}
	if scores[DimCommunication] < 70 {
		out = append(out, "控制单题时长在 90-150 秒：超过这个长度说明你在铺陈而不是回答，先用 20 秒给结论。")
	}
	if scores[DimFit] < 70 {
		out = append(out, "面试前把 JD 拆成要求清单，每场准备 3 个能直接对应岗位要求的项目故事。")
	}
	if scores[DimAccuracy] < 70 {
		out = append(out, "对不确定的事实明确说\"这块我不确定\"，再给出你的推测——硬编错误答案的代价远大于承认不知道。")
	}
	return out
}

func learningPlan(ranked []Dimension, scores map[string]float64) []LearningItem {
	// The two weakest axes, weakest last, capped at 3 items.
	worst := append([]Dimension(nil), ranked...)
	for i, j := 0, len(worst)-1; i < j; i, j = i+1, j-1 {
		worst[i], worst[j] = worst[j], worst[i]
	}
	items := []LearningItem{}
	for _, d := range worst {
		if len(items) >= 3 {
			break
		}
		if scores[d.Key] >= 80 {
			break // nothing left worth calling a study plan
		}
		items = append(items, LearningItem{
			Topic: d.Label,
			Why:   fmt.Sprintf("本场得分 %.0f 分，是拉低整体评价的主要维度之一", scores[d.Key]),
			How:   dimensionAdvice(d.Key),
		})
	}
	if len(items) == 0 {
		items = append(items, LearningItem{
			Topic: "差异化亮点",
			Why:   "各维度都已达标，进一步提升需要能拉开差距的深度亮点",
			How:   "挑一个你最有兴趣的方向做一个可展示的产出（开源贡献、性能优化案例、技术分享），并在面试中主动引导到这个话题",
		})
	}
	return items
}
