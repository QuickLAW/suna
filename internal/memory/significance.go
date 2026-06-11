package memory

import (
	"regexp"
	"strings"
	"unicode"
)

type Significance string

const (
	SignificanceHigh   Significance = "high"
	SignificanceMedium Significance = "medium"
	SignificanceLow    Significance = "low"
)

const (
	MemoryKindCommunication = "communication"
	MemoryKindWorkflow      = "workflow"
	MemoryKindPreference    = "preference"
	MemoryKindConstraint    = "constraint"
	MemoryKindCorrection    = "correction"
	MemoryKindUserFact      = "user_fact"

	MemorySourceExplicit   = "explicit"
	MemorySourceInferred   = "inferred"
	MemorySourceCorrection = "correction"
)

type Candidate struct {
	UserID       string
	Kind         string
	Content      string
	Tags         []string
	Source       string
	Confidence   float64
	Evidence     string
	Significance Significance
}

/*
ExtractCandidate 从用户消息中提取“长期用户画像候选”。

这里故意只处理用户输入，不处理 assistant 总结或 tool 结果：长期画像必须来自用户信号，
否则当前任务的实现细节、项目状态和 agent 自我总结很容易污染跨会话记忆。
*/
func ExtractCandidate(input string, userCorrection bool) (Candidate, bool) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)
	if trimmed == "" || isTrivialInput(lower) {
		return Candidate{}, false
	}

	if userCorrection {
		return normalizeCandidate(Candidate{Kind: MemoryKindCorrection, Content: profileContent(trimmed), Tags: []string{"correction"}, Source: MemorySourceCorrection, Confidence: 0.9, Evidence: trimmed, Significance: SignificanceHigh})
	}
	if isExplicitRemember(lower) {
		if looksTaskSpecific(lower) {
			return Candidate{}, false
		}
		return normalizeCandidate(Candidate{Kind: inferCandidateKind(lower), Content: profileContent(trimmed), Tags: inferCandidateTags(lower), Source: MemorySourceExplicit, Confidence: 0.95, Evidence: trimmed, Significance: SignificanceHigh})
	}
	if looksTaskSpecific(lower) {
		return Candidate{}, false
	}
	if containsDurablePreference(lower) {
		return normalizeCandidate(Candidate{Kind: inferCandidateKind(lower), Content: profileContent(trimmed), Tags: inferCandidateTags(lower), Source: MemorySourceInferred, Confidence: 0.75, Evidence: trimmed, Significance: SignificanceMedium})
	}
	return Candidate{}, false
}

func normalizeCandidate(c Candidate) (Candidate, bool) {
	c.Kind = normalizeKind(c.Kind)
	c.Source = normalizeSource(c.Source)
	c.Content = strings.TrimSpace(c.Content)
	c.Evidence = truncateRunes(strings.TrimSpace(c.Evidence), 180)
	c.Tags = normalizeTags(c.Tags)
	if c.Confidence <= 0 {
		c.Confidence = 0.7
	}
	if c.Confidence > 1 {
		c.Confidence = 1
	}
	if c.Significance == "" {
		c.Significance = SignificanceMedium
	}
	if c.Content == "" {
		return Candidate{}, false
	}
	return c, true
}

func inferCandidateKind(input string) string {
	switch {
	case containsAny(input, "回复", "简短", "详细", "中文", "english", "tone", "reply"):
		return MemoryKindCommunication
	case containsAny(input, "不要", "别", "不希望", "不能", "禁止", "avoid", "never"):
		return MemoryKindConstraint
	case containsAny(input, "流程", "先", "下次", "以后", "步骤", "workflow"):
		return MemoryKindWorkflow
	case containsAny(input, "我是", "我在用", "我的电脑", "my "):
		return MemoryKindUserFact
	case containsAny(input, "错", "纠正", "不是", "不对", "correct"):
		return MemoryKindCorrection
	default:
		return MemoryKindPreference
	}
}

func inferCandidateTags(input string) []string {
	var tags []string
	pairs := []struct{ needle, tag string }{
		{"代码", "coding"}, {"测试", "testing"}, {"架构", "architecture"}, {"设计", "design"},
		{"调试", "debugging"}, {"日志", "debugging"}, {"报错", "debugging"},
		{"工具", "tools"}, {"tool", "tools"}, {"tui", "tui"}, {"ui", "tui"},
		{"安全", "security"}, {"敏感", "security"}, {"隐私", "privacy"},
		{"中文", "communication"}, {"简短", "communication"}, {"回复", "communication"},
	}
	for _, p := range pairs {
		if strings.Contains(input, p.needle) {
			tags = append(tags, p.tag)
		}
	}
	return tags
}

func profileContent(input string) string {
	input = strings.TrimSpace(input)
	for _, p := range rememberPatterns {
		input = strings.TrimSpace(strings.TrimPrefix(input, p))
	}
	return truncateRunes(input, 120)
}

func looksTaskSpecific(input string) bool {
	if containsAny(input, "改吧", "可行", "继续", "帮我", "修复", "实现", "新增", "删除", "检查", "看看", "跑测试", "看这个文件", "截图", "这次", "当前", "现在这个") {
		return true
	}
	if pathLikePattern.MatchString(input) || strings.Contains(input, "http://") || strings.Contains(input, "https://") {
		return true
	}
	return false
}

var pathLikePattern = regexp.MustCompile(`(?i)(/[^\s]+|[a-z0-9_-]+\.(go|md|toml|json|yaml|yml|ts|tsx|js|jsx|py))`)

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

var rememberPatterns = []string{
	"记住", "帮我记住", "以后都", "以后都这样", "以后不要再", "以后别再",
	"always", "never", "remember", "from now on", "keep in mind",
}

var durablePatterns = []string{
	"我希望", "我不希望", "我更", "我比较", "我倾向", "我的习惯", "我的性格",
	"下次", "以后", "别再", "不要再", "更喜欢", "不喜欢",
	"i want", "i don't want", "i tend to", "next time", "avoid", "prefer",
}

func isExplicitRemember(input string) bool {
	for _, p := range rememberPatterns {
		if strings.Contains(input, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func containsDurablePreference(input string) bool {
	for _, p := range durablePatterns {
		if strings.Contains(input, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

var trivialInputs = map[string]bool{
	"好": true, "好的": true, "ok": true, "okay": true,
	"继续": true, "嗯": true, "对": true, "是": true,
	"yes": true, "yeah": true, "yep": true, "sure": true,
	"continue": true, "go": true, "go on": true,
	"谢谢": true, "thanks": true, "thx": true,
	"没问题": true, "算了": true, "不了": true,
	"no": true, "nope": true, "不用": true,
}

func isTrivialInput(input string) bool {
	if trivialInputs[input] {
		return true
	}
	trimmed := strings.TrimFunc(input, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
	if trivialInputs[trimmed] {
		return true
	}
	if len([]rune(input)) <= 3 && !containsCJK(input) {
		return true
	}
	return false
}

func containsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			return true
		}
	}
	return false
}
