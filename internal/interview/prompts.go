package interview

import (
	"fmt"
	"strings"

	"github.com/Potterluo/dream-interviewer/internal/llm"
)

// prompts.go builds the three model conversations the interviewer runs.
//
// The load-bearing convention here is the two-part reply: for the stages
// a candidate watches (点评, 总评) the model writes the human-facing prose
// FIRST, then the marker line, then strict JSON. That lets the SSE stream
// forward prose token-by-token while the structured half is buffered and
// parsed at the end — one call gives both a live experience and typed
// data. Plan generation is machine-only, so it asks for bare JSON.
//
// The rubric language deliberately mirrors the reference project's
// formula (得分 = 展现能力 / 期望能力 × 100) so scores mean the same thing
// here as they did there.
const jsonDelimiter = "<<<JSON>>>"

// PlanMessages asks for the question script. Pure JSON, no prose: nothing
// about planning is worth showing the candidate.
func PlanMessages(spec Spec) []llm.Message {
	var b strings.Builder

	fmt.Fprintf(&b, "你是一位拥有十年经验的资深面试官，擅长为不同岗位设计结构化模拟面试。\n")
	fmt.Fprintf(&b, "现在请为下面这场模拟面试设计题目大纲。\n\n")

	fmt.Fprintf(&b, "【目标岗位】%s\n", spec.Role)
	fmt.Fprintf(&b, "【候选人级别】%s\n", LabelOf("level", spec.Level))
	fmt.Fprintf(&b, "【面试类型】%s\n", LabelOf("type", spec.Type))
	fmt.Fprintf(&b, "【难度】%s\n", LabelOf("difficulty", spec.Difficulty))
	fmt.Fprintf(&b, "【题目数量】%d 题\n\n", spec.QuestionCount)

	b.WriteString("【岗位描述 JD】\n")
	b.WriteString(orPlaceholderString(spec.JD, "（候选人未提供，请依据岗位名称的通用要求设计）"))
	b.WriteString("\n\n【候选人简历】\n")
	b.WriteString(orPlaceholderString(spec.Resume, "（候选人未提供，请设计不依赖具体经历的通用题目）"))
	b.WriteString("\n\n")

	b.WriteString("【本题面试使用的评分维度（必须原样使用这些 key）】\n")
	b.WriteString(dimensionSpec(DimensionsFor(spec)))
	b.WriteString("\n")

	b.WriteString("【设计原则】\n")
	b.WriteString("1. 题目必须紧扣岗位描述与简历中的具体信息（技术栈、项目、业务领域），禁止出现\"请介绍一下你自己\"这类空泛问题。\n")
	b.WriteString("2. 难度与候选人级别匹配：初级考基础与动手能力，高级考原理、边界条件、失败模式与工程取舍。\n")
	if spec.Type == TypeBehavior {
		b.WriteString("3. 行为面试题请采用 STAR 追问思路与\"宝洁八问\"式的具体情境，要求候选人给出真实事例而非观点。\n")
	} else {
		b.WriteString("3. 每题都要有明确的正确答案范围，便于客观打分；避免\"你怎么看\"式的主观题。\n")
	}
	b.WriteString("4. 题目之间不要重复考察同一个点，覆盖各个评分维度。\n")
	b.WriteString("5. 每题的 reference 写清\"优秀回答应覆盖的要点\"，2-4 句即可。\n")
	b.WriteString("6. 为每道题准备 1-2 个 followUps（追问方向），用于候选人回答不足时深挖。\n\n")

	b.WriteString(languageRule(spec))
	b.WriteString("\n只输出一个 JSON 对象，不要输出任何解释文字，不要使用 markdown 代码块。JSON 结构如下：\n")
	b.WriteString(planSchema(spec))

	return []llm.Message{
		llm.System(planSystemPrompt),
		llm.User(b.String()),
	}
}

const planSystemPrompt = "你是一位资深面试官与面试命题专家。你严格遵守输出的 JSON 格式要求，" +
	"只输出合法 JSON，不输出任何多余文字。你设计的题目具体、专业、可客观评分。"

// GradeMessages asks for the two-part verdict on one answer: streamed
// prose for the candidate, then the grade JSON.
func GradeMessages(in GradeInput) []llm.Message {
	spec := in.Spec
	dims := in.Plan.Dimensions
	if len(dims) == 0 {
		dims = DimensionsFor(spec)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "你是这场模拟面试的面试官，岗位是【%s】（%s）。\n\n",
		spec.Role, LabelOf("level", spec.Level))

	b.WriteString("【评分维度（必须原样使用这些 key）】\n")
	b.WriteString(dimensionSpec(dims))
	b.WriteString("\n")

	fmt.Fprintf(&b, "【正在考察的题目】%s\n", in.Question.Question)
	if in.Question.Intent != "" {
		fmt.Fprintf(&b, "【本题考察点】%s\n", in.Question.Intent)
	}
	if in.Question.Reference != "" {
		fmt.Fprintf(&b, "【出题人预设的答案要点】%s\n", in.Question.Reference)
	}
	if in.Kind == KindFollowup {
		b.WriteString("【说明】这是一道追问，评分时请一并参考候选人之前的回答。\n")
	}
	b.WriteString("\n")

	if len(in.History) > 0 {
		b.WriteString("【此前问答记录（供判断进步/一致性与出追问题）】\n")
		b.WriteString(transcript(in.History))
		b.WriteString("\n")
	}

	b.WriteString("【候选人本题回答】\n")
	if strings.TrimSpace(in.Answer) == "" {
		b.WriteString("（候选人跳过/未作答）\n")
	} else {
		b.WriteString(in.Answer)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("【评分方法】\n")
	b.WriteString("每个维度：得分 = 候选人展现的能力 / 该题期望的能力 × 100，取 0-100 的整数。\n")
	b.WriteString("总分 score = 各维度得分按其权重加权平均，取 0-100（可带一位小数）。\n")
	fmt.Fprintf(&b, "级别基准：%s，判断\"期望的能力\"时要按这个级别来。\n", levelExpectation(spec.Level))
	b.WriteString("未作答或答非所问时，务必给出低分并说明，不要为了鼓励而虚高给分。\n\n")

	b.WriteString("【输出格式】\n")
	fmt.Fprintf(&b, "第一部分：给候选人的点评（Markdown，150-300 字，直接称呼\"你\"）。"+
		"先明确指出回答中做对的地方，再指出具体缺口，必须引用候选人原话或明确指出缺失的关键点，"+
		"不要写\"建议多练习\"这类空话。\n")
	fmt.Fprintf(&b, "第二部分：单独一行输出 %s，其后只输出 JSON 对象（不要 markdown 代码块）。\n\n", jsonDelimiter)

	fmt.Fprintf(&b, "JSON 结构：\n")
	fmt.Fprintf(&b, `{
  "score": 82.0,
  "dimensions": {%s},
  "strengths": ["具体做对的点，1-3 条"],
  "weaknesses": ["具体缺口，1-3 条"],
  "feedback": "与第一部分同样的点评正文",
  "reference": "本题优秀回答的要点，2-4 句",
  "followUp": "%s",
  "verdict": "strong | ok | weak"
}`, dimensionKeysJSON(dims), followUpInstruction(in))

	b.WriteString("\n\n")
	b.WriteString(languageRule(spec))

	return []llm.Message{
		llm.System(gradeSystemPrompt),
		llm.User(b.String()),
	}
}

const gradeSystemPrompt = "你是一位严格但公正的面试官。你按维度客观打分，点评具体、可执行，" +
	"既指出真实问题也承认做得好的地方。你遵守要求的输出格式。"

// ReportMessages asks for the final report: streamed 总评 then the report
// JSON.
func ReportMessages(in ReportInput) []llm.Message {
	spec := in.Spec
	dims := in.Plan.Dimensions
	if len(dims) == 0 {
		dims = DimensionsFor(spec)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "你是这场模拟面试的面试官。面试已经结束，请给出最终评估报告。\n\n")
	fmt.Fprintf(&b, "【目标岗位】%s\n【候选人级别】%s\n【面试类型】%s\n",
		spec.Role, LabelOf("level", spec.Level), LabelOf("type", spec.Type))

	b.WriteString("【评分维度（必须原样使用这些 key）】\n")
	b.WriteString(dimensionSpec(dims))
	b.WriteString("\n")

	if spec.JD != "" {
		b.WriteString("【岗位描述 JD】\n")
		b.WriteString(TrimTo(spec.JD, 1500))
		b.WriteString("\n\n")
	}
	if spec.Resume != "" {
		b.WriteString("【候选人简历】\n")
		b.WriteString(TrimTo(spec.Resume, 1500))
		b.WriteString("\n\n")
	}

	b.WriteString("【全部问答记录】\n")
	b.WriteString(transcript(in.Turns))
	b.WriteString("\n")

	b.WriteString("【评分方法】\n")
	b.WriteString("每个维度：该维度下各题得分的加权平均（0-100）。\n")
	b.WriteString("overallScore = 各维度得分按权重加权平均（0-100，一位小数）。\n")
	b.WriteString("hrSatisfaction = 预测 HR 初筛通过的概率（0-100）：它衡量简历与表达给 HR 的第一印象，" +
		"与技术得分不完全一致，请独立判断。\n\n")

	b.WriteString("【要求】\n")
	b.WriteString("1. 所有结论必须能追溯到上面的具体问答，禁止编造候选人没说过的经历。\n")
	b.WriteString("2. strengths / weaknesses / highlights 要具体，highlights 直接引用候选人原话。\n")
	b.WriteString("3. resumeSuggestions 针对简历本身（表述、量化、技术栈呈现、与岗位的匹配度）给出可立即修改的建议。\n")
	b.WriteString("4. interviewStrategies 针对面试表现（答题结构、深度、沟通）给出下一次面试就能用的策略。\n")
	b.WriteString("5. learningPlan 给出 2-4 个具体学习项，how 要能直接执行（看什么、练什么）。\n")
	b.WriteString("6. recommendation 只能是 strong_hire / hire / maybe / no_hire。\n")
	if in.Plan.Rationale != "" {
		fmt.Fprintf(&b, "7. 本场命题思路（供参考）：%s\n", TrimTo(in.Plan.Rationale, 400))
	}
	b.WriteString("\n")

	b.WriteString("【输出格式】\n")
	b.WriteString("第一部分：给候选人的总评（Markdown，200-400 字），先给整体结论，再讲亮点与主要问题，最后给方向。\n")
	fmt.Fprintf(&b, "第二部分：单独一行输出 %s，其后只输出 JSON 对象（不要 markdown 代码块）。\n\n", jsonDelimiter)

	fmt.Fprintf(&b, `JSON 结构：
{
  "overallScore": 78.5,
  "hrSatisfaction": 74,
  "dimensions": [{"key": "%s", "score": 80, "comment": "该维度的具体评价"}],
  "summary": "与第一部分同样的总评正文",
  "strengths": ["整体优势，2-4 条"],
  "weaknesses": ["主要短板，2-4 条"],
  "suggestions": ["改进建议，2-4 条"],
  "resumeSuggestions": ["简历改进建议，2-4 条"],
  "interviewStrategies": ["面试策略，2-4 条"],
  "learningPlan": [{"topic": "主题", "why": "为什么学", "how": "怎么学"}],
  "recommendation": "strong_hire | hire | maybe | no_hire",
  "recommendationReason": "给出一句话理由",
  "highlights": ["候选人原话摘录"],
  "risks": ["需要面试官警惕的风险点，没有就给空数组"]
}`, dimKeyAt(dims, 0))

	b.WriteString("\n\n")
	b.WriteString(languageRule(spec))

	return []llm.Message{
		llm.System(reportSystemPrompt),
		llm.User(b.String()),
	}
}

const reportSystemPrompt = "你是一位资深面试官与职业发展顾问。你的评估客观、具体、对候选人有实际帮助，" +
	"既不客套也不打击。你遵守要求的输出格式。"

// TestMessages is the tiny probe behind POST /api/interview/engine/test.
func TestMessages() []llm.Message {
	return []llm.Message{
		llm.System("You are a connectivity probe. Reply with exactly the single word PONG and nothing else."),
		llm.User("ping"),
	}
}

// --- prompt fragments -------------------------------------------------------

func dimensionSpec(dims []Dimension) string {
	var b strings.Builder
	for _, d := range dims {
		fmt.Fprintf(&b, "- %s（%s），权重 %.2f", d.Key, d.Label, d.Weight)
		if d.Desc != "" {
			fmt.Fprintf(&b, "：%s", d.Desc)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func dimensionKeysJSON(dims []Dimension) string {
	parts := make([]string, 0, len(dims))
	for _, d := range dims {
		parts = append(parts, fmt.Sprintf("%q: 85", d.Key))
	}
	return strings.Join(parts, ", ")
}

func dimKeyAt(dims []Dimension, i int) string {
	if i < len(dims) {
		return dims[i].Key
	}
	return DimProfessional
}

// transcript renders the running Q&A as compact prose the model can use
// as evidence. Answers are truncated: the point is context, not a full
// replay, and long transcripts are the main cost driver.
func transcript(turns []TurnBrief) string {
	if len(turns) == 0 {
		return "（暂无）\n"
	}
	var b strings.Builder
	for _, t := range turns {
		kind := "问题"
		if t.Kind == KindFollowup {
			kind = "追问"
		}
		fmt.Fprintf(&b, "%d. [%s]%s\n", t.Seq, kind, TrimTo(t.Question, 300))
		if strings.TrimSpace(t.Answer) == "" {
			b.WriteString("   回答：（未作答）\n")
		} else {
			fmt.Fprintf(&b, "   回答：%s\n", TrimTo(t.Answer, 1200))
		}
		if t.Score > 0 {
			fmt.Fprintf(&b, "   得分：%.0f\n", t.Score)
		}
	}
	return b.String()
}

// followUpInstruction encodes the follow-up budget. Adaptive follow-ups
// are what make this feel like an interview rather than a quiz, but an
// unbounded interviewer never finishes, so the budget is explicit.
func followUpInstruction(in GradeInput) string {
	const (
		maxPerQuestion  = 1
		maxPerInterview = 3
	)
	if in.Kind == KindFollowup {
		return "本次不再追问，留空字符串"
	}
	if in.FollowUps >= maxPerInterview {
		return "本场追问次数已达上限，留空字符串"
	}
	return fmt.Sprintf(
		"如果候选人的回答里有值得深挖的具体点（例如提到但没讲清的技术细节、含糊的量化结果、可疑的因果关系），给出一个追问（1 句、开放式、不要提示答案）；否则留空字符串。每题最多追问 %d 次",
		maxPerQuestion)
}

func languageRule(spec Spec) string {
	if spec.Language == LanguageEN {
		return "请全程使用英文输出（包括点评、题目与 JSON 中的文本字段）。"
	}
	return "请全程使用简体中文输出（包括点评、题目与 JSON 中的文本字段）。"
}

func levelExpectation(level string) string {
	switch level {
	case LevelJunior:
		return "初级：能写、能跑、能说清基本概念，允许经验不足"
	case LevelSenior:
		return "高级：要求原理级理解、能处理边界与失败场景、有明确的工程取舍"
	case LevelExpert:
		return "资深/专家：要求体系化认知、能定义问题、能权衡长期成本并影响他人"
	default:
		return "中级：能独立完成常规任务，理解常用方案的原理与适用边界"
	}
}

func orPlaceholderString(s, placeholder string) string {
	if strings.TrimSpace(s) == "" {
		return placeholder
	}
	return TrimTo(s, 3000)
}

func planSchema(spec Spec) string {
	dims := DimensionsFor(spec)
	var b strings.Builder
	b.WriteString(`{
  "dimensions": [`)
	for i, d := range dims {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `{"key": %q, "label": %q, "weight": %.2f}`, d.Key, d.Label, d.Weight)
	}
	b.WriteString(`],
  "rationale": "一句话说明本场命题思路",
  "questions": [
    {
      "seq": 1,
      "dimension": "` + dimKeyAt(dims, 0) + `",
      "question": "题目正文",
      "intent": "本题想考察什么",
      "weight": 1.0,
      "followUps": ["追问方向 1"],
      "reference": "优秀回答应覆盖的要点"
    }
  ]
}`)
	fmt.Fprintf(&b, "\n\nquestions 数组必须正好包含 %d 道题，seq 从 1 连续编号，dimension 必须取自上面的 key 列表。",
		spec.QuestionCount)
	return b.String()
}
