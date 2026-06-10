package model

import "strings"

const sessionStatePrefix = "<session_state>"

// FormatSessionStateForModel 只负责把独立 SessionState 包装成 provider 可发送的内部上下文。
// 它不进入 system prompt，也不写回 WorkingMemory，避免污染稳定前缀缓存和可见聊天历史。
func FormatSessionStateForModel(state string) string {
	state = strings.TrimSpace(state)
	if state == "" {
		return ""
	}
	return sessionStatePrefix + "\n" +
		"This is internal session memory for continuity, not a user request. Current user instructions override it.\n\n" +
		state + "\n</session_state>"
}
