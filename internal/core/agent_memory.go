package core

import (
	"context"
	"strings"

	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
	"github.com/alanchenchen/suna/internal/tool"
)

func (a *Agent) enqueueMemoryEvent(ctx context.Context, role model.Role, content string, hadToolCall, toolFailed, guardBlocked, userCorrection bool) {
	if a.extractQueue == nil || content == "" {
		return
	}
	// 主链路只做零成本规则判断：低价值内容直接丢弃，避免把普通聊天都送进
	// daemon 的记忆整理 LLM，控制成本和记忆噪音。
	sig := memory.JudgeSignificance(content, "", hadToolCall, toolFailed, guardBlocked, userCorrection)
	if sig == memory.SignificanceLow {
		return
	}
	a.extractQueue.Push(ctx, memory.DefaultUserID, string(role), content, sig)
}

func (a *Agent) saveConversationState(ctx context.Context) {
	if a.conversation == nil || a.working == nil {
		return
	}
	msgs := a.working.Messages()
	// conversation_state 是“恢复上一轮会话”的同步快照，不依赖异步 memory_queue。
	// last_messages 会恢复进 LLM 上下文；toolSummary 只给 TUI 展示，不恢复给模型。
	_ = a.conversation.Save(ctx, memory.DefaultUserID, memory.BuildResumeSummary(msgs), msgs, a.toolSummary)
}

func (a *Agent) addToolSummary(name string, result tool.Result) {
	if name == "" {
		return
	}
	status := "success"
	if result.IsError {
		status = "error"
	}
	// 只保留工具结果的短摘要用于恢复 UI 展示；不保存 raw args/stdout/stderr，
	// 避免 SQL 膨胀、敏感信息泄漏和恢复后上下文污染。
	summary := summarizeToolResult(result.Content)
	if summary == "" {
		summary = "completed"
	}
	a.toolSummary = append(a.toolSummary, memory.ToolSummaryItem{Name: name, Status: status, Summary: summary})
}

func summarizeToolResult(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	compact := make([]string, 0, 2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		compact = append(compact, line)
		if len(compact) >= 2 {
			break
		}
	}
	if len(compact) == 0 {
		return ""
	}
	return strings.Join(compact, " | ")
}
