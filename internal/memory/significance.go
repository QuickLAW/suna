package memory

import (
	"strings"
	"unicode"
)

type Significance string

const (
	SignificanceHigh   Significance = "high"
	SignificanceMedium Significance = "medium"
	SignificanceLow    Significance = "low"
)

/*
JudgeSignificance 判断交互的显著性等级（零 LLM 成本）。

		高显著性（立即触发提取）：

		  - 用户说"记住"/"以后都这样"/"以后不要再这样"等明确长期指令

	  - 工具执行失败

	  - Guard 拦截了操作

	  - 用户纠正了 agent 的输出

	    中显著性（正常排队）：

	  - 用户表达了可能长期有效的偏好、习惯或边界

低显著性（跳过提取）：
  - 纯闲聊 / 简单问候
  - 用户只回复"好"/"继续"/"OK"
  - 单轮信息查询
*/
func JudgeSignificance(userInput, agentOutput string, hadToolCall, toolFailed, guardBlocked, userCorrection bool) Significance {
	if guardBlocked || userCorrection || toolFailed {
		return SignificanceHigh
	}

	userLower := strings.ToLower(strings.TrimSpace(userInput))
	if isExplicitRemember(userLower) {
		return SignificanceHigh
	}

	if isTrivialInput(userLower) {
		return SignificanceLow
	}

	if containsDurablePreference(userLower) {
		return SignificanceMedium
	}

	return SignificanceLow
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
