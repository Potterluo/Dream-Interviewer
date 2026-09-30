package interview

import (
	"math"
	"strings"
	"time"
)

// nowMs is the millisecond clock used for latency reporting. Kept as a
// named function so tests can reason about it and so the two call sites
// in engine.Test cannot drift apart.
func nowMs() int64 { return time.Now().UnixMilli() }

// dims.go: the scoring rubric — which axes an interview is judged on,
// and the small numeric helpers every stage shares.

// DimensionsFor returns the dimension set for an interview type. Weights
// sum to 1.0. The UI renders these labels directly, and the prompts are
// built from them, so the rubric is defined in exactly one place.
func DimensionsFor(spec Spec) []Dimension {
	switch spec.Type {
	case TypeBehavior:
		return []Dimension{
			{Key: DimSituational, Label: "情境应对", Weight: 0.30, Desc: "面对真实工作场景时的判断与取舍"},
			{Key: DimTeamwork, Label: "团队协作", Weight: 0.25, Desc: "跨角色配合、冲突处理、推动落地的能力"},
			{Key: DimSelfAwareness, Label: "自我认知", Weight: 0.20, Desc: "对自身优势、短板与成长路径的清醒程度"},
			{Key: DimCommunication, Label: "沟通表达", Weight: 0.25, Desc: "结构化表达、重点突出、倾听与确认"},
		}
	case TypeSystemDesign:
		return []Dimension{
			{Key: DimAccuracy, Label: "架构正确性", Weight: 0.30, Desc: "方案是否满足需求与约束，关键取舍是否成立"},
			{Key: DimDepth, Label: "技术深度", Weight: 0.25, Desc: "对容量、一致性、可用性等硬指标的把握"},
			{Key: DimProblemSolving, Label: "问题解决", Weight: 0.25, Desc: "拆解模糊需求、逐层收敛到可落地方案"},
			{Key: DimCommunication, Label: "沟通表达", Weight: 0.20, Desc: "边画边讲、主动澄清、控制讨论范围"},
		}
	case TypeTech:
		return []Dimension{
			{Key: DimAccuracy, Label: "技术准确性", Weight: 0.30, Desc: "概念、原理与事实性描述的准确程度"},
			{Key: DimDepth, Label: "技术深度", Weight: 0.25, Desc: "能否深入到原理、边界条件与失败模式"},
			{Key: DimProblemSolving, Label: "问题解决", Weight: 0.25, Desc: "分析路径、权衡取舍与工程判断"},
			{Key: DimCommunication, Label: "沟通表达", Weight: 0.20, Desc: "结构化表达、术语准确、重点突出"},
		}
	default: // TypeMixed
		return []Dimension{
			{Key: DimProfessional, Label: "专业能力", Weight: 0.30, Desc: "岗位所需的专业知识与实操经验"},
			{Key: DimLogic, Label: "逻辑思维", Weight: 0.25, Desc: "推理链条是否完整、结论是否有依据"},
			{Key: DimCommunication, Label: "沟通表达", Weight: 0.25, Desc: "结构化表达、重点突出、倾听与确认"},
			{Key: DimFit, Label: "岗位匹配", Weight: 0.20, Desc: "经历与目标岗位要求的契合程度"},
		}
	}
}

// DimensionByKey finds a dimension in a set, returning a zero value and
// false when the key is unknown (e.g. a hallucinated dimension name in
// an LLM reply).
func DimensionByKey(dims []Dimension, key string) (Dimension, bool) {
	for _, d := range dims {
		if d.Key == key {
			return d, true
		}
	}
	return Dimension{}, false
}

// NormalizeDimensions rewrites weights so they sum to 1.0. Zero or
// negative weights are treated as "unspecified" and share what is left
// evenly; if every weight is unusable they all become equal.
func NormalizeDimensions(dims []Dimension) []Dimension {
	if len(dims) == 0 {
		return dims
	}
	out := make([]Dimension, len(dims))
	copy(out, dims)
	total := 0.0
	for _, d := range out {
		if d.Weight > 0 {
			total += d.Weight
		}
	}
	if total <= 0 {
		even := 1.0 / float64(len(out))
		for i := range out {
			out[i].Weight = even
		}
		return out
	}
	for i := range out {
		if out[i].Weight > 0 {
			out[i].Weight = out[i].Weight / total
		}
	}
	return out
}

// WeightedScore combines per-dimension scores (0-100) using the
// dimension weights, ignoring dimensions with no score. Returns 0 when
// nothing is scored.
func WeightedScore(dims []Dimension, scores map[string]float64) float64 {
	total, weight := 0.0, 0.0
	for _, d := range dims {
		s, ok := scores[d.Key]
		if !ok {
			continue
		}
		w := d.Weight
		if w <= 0 {
			w = 1
		}
		total += clamp(s, 0, 100) * w
		weight += w
	}
	if weight == 0 {
		return 0
	}
	return round1(total / weight)
}

// --- small helpers ----------------------------------------------------------

func trim(s string) string { return strings.TrimSpace(s) }

// oneOf returns v when it is in allowed, else fallback.
func oneOf(v string, allowed []string, fallback string) string {
	v = strings.TrimSpace(v)
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return fallback
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// round1 rounds to one decimal — scores are display data, and 78.5 reads
// better than 78.49999999999999 in both JSON and the UI.
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// TrimTo shortens s to at most n runes, appending an ellipsis. Rune-safe
// so Chinese résumé snippets do not get cut mid-character.
func TrimTo(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// uniqStrings removes duplicates and blanks while preserving order.
func uniqStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// strSlice guarantees a non-nil slice so JSON shows [] instead of null,
// which the frontend relies on when mapping over arrays.
func strSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
