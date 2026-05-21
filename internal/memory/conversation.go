package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/alanchenchen/suna/internal/model"
)

type ConversationState struct {
	UserID            string
	ResumeSummary     string
	LastMessages      []model.Message
	ToolSummary       []ToolSummaryItem
	MemoryProcessedAt time.Time
	UpdatedAt         time.Time
}

type ToolSummaryItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type ConversationStore struct {
	db *sql.DB
}

func NewConversationStore(db *sql.DB) *ConversationStore { return &ConversationStore{db: db} }

func (s *ConversationStore) Load(ctx context.Context, userID string) (*ConversationState, error) {
	if userID == "" {
		userID = DefaultUserID
	}
	row := s.db.QueryRowContext(ctx, `SELECT user_id, resume_summary, last_messages, tool_summary, memory_processed_at, updated_at FROM conversation_state WHERE user_id = ?`, userID)
	var st ConversationState
	var lastMessages, toolSummary string
	var processed, updated sql.NullString
	if err := row.Scan(&st.UserID, &st.ResumeSummary, &lastMessages, &toolSummary, &processed, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(lastMessages), &st.LastMessages)
	_ = json.Unmarshal([]byte(toolSummary), &st.ToolSummary)
	st.MemoryProcessedAt = parseDBTime(processed.String)
	st.UpdatedAt = parseDBTime(updated.String)
	return &st, nil
}

func (s *ConversationStore) Save(ctx context.Context, userID, summary string, msgs []model.Message, tools []ToolSummaryItem) error {
	if userID == "" {
		userID = DefaultUserID
	}
	// last_messages 是恢复上一轮会话的同步快照，只保存用户可见的 user/assistant transcript。
	// tool/raw/system 消息不进入这里，避免把内部执行轨迹当作对话历史恢复给模型。
	msgs = visibleMessages(msgs)
	msgJSON, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	// tool_summary 只服务 TUI 恢复展示，不进入 LLM 上下文，也不进入长期 user_memory。
	toolJSON, err := json.Marshal(normalizeToolSummary(tools))
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO conversation_state (user_id, resume_summary, last_messages, tool_summary, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			resume_summary = excluded.resume_summary,
			last_messages = excluded.last_messages,
			tool_summary = excluded.tool_summary,
			updated_at = excluded.updated_at`, userID, summary, string(msgJSON), string(toolJSON), now)
	return err
}

func (s *ConversationStore) ClearLastMessages(ctx context.Context, userID string) error {
	if userID == "" {
		userID = DefaultUserID
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_state (user_id, resume_summary, last_messages, tool_summary, updated_at)
		VALUES (?, '', '[]', '[]', ?)
		ON CONFLICT(user_id) DO UPDATE SET resume_summary = '', last_messages = '[]', tool_summary = '[]', updated_at = excluded.updated_at`, userID, now)
	return err
}

func lastVisibleMessages(msgs []model.Message, limit int) []model.Message {
	if limit <= 0 {
		limit = 2
	}
	visible := make([]model.Message, 0, limit)
	for i := len(msgs) - 1; i >= 0 && len(visible) < limit; i-- {
		if msgs[i].Role == model.RoleUser || msgs[i].Role == model.RoleAssistant {
			text := strings.TrimSpace(msgs[i].Text())
			if text == "" {
				continue
			}
			visible = append(visible, model.NewTextMessage(msgs[i].Role, text))
		}
	}
	for i, j := 0, len(visible)-1; i < j; i, j = i+1, j-1 {
		visible[i], visible[j] = visible[j], visible[i]
	}
	return visible
}

func visibleMessages(msgs []model.Message) []model.Message {
	visible := make([]model.Message, 0, len(msgs))
	for _, m := range msgs {
		// 恢复会话只还原用户和 Suna 的可见对话；tool result 在当前会话内可见给模型，
		// 但恢复后不再注入，避免过期工具状态污染新的 LLM 请求。
		if m.Role == model.RoleUser || m.Role == model.RoleAssistant {
			text := strings.TrimSpace(m.Text())
			if text == "" {
				continue
			}
			// 重新构造成纯文本消息，避免把 assistant tool_calls/raw 结构写入恢复快照。
			visible = append(visible, model.NewTextMessage(m.Role, text))
		}
	}
	return visible
}

func BuildResumeSummary(msgs []model.Message) string {
	visible := lastVisibleMessages(msgs, 2)
	if len(visible) == 0 {
		return ""
	}
	var lastUser, lastAssistant string
	for _, m := range visible {
		switch m.Role {
		case model.RoleUser:
			lastUser = truncateRunes(m.Text(), 180)
		case model.RoleAssistant:
			lastAssistant = truncateRunes(m.Text(), 220)
		}
	}
	if lastUser != "" && lastAssistant != "" {
		return "上一轮用户说: " + lastUser + "\n上一轮 Suna 回复: " + lastAssistant
	}
	if lastUser != "" {
		return "上一轮用户说: " + lastUser
	}
	return "上一轮 Suna 回复: " + lastAssistant
}

func normalizeToolSummary(items []ToolSummaryItem) []ToolSummaryItem {
	out := make([]ToolSummaryItem, 0, len(items))
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		item.Status = strings.TrimSpace(item.Status)
		item.Summary = strings.TrimSpace(item.Summary)
		if item.Name == "" || item.Summary == "" {
			continue
		}
		if item.Status == "" {
			item.Status = "done"
		}
		// 摘要只用于恢复 UI 的“上一轮操作摘要”块，必须短，不能变成另一个历史日志。
		if len([]rune(item.Summary)) > 180 {
			item.Summary = truncateRunes(item.Summary, 180)
		}
		out = append(out, item)
	}
	return out
}

func FormatToolSummary(items []ToolSummaryItem) string {
	items = normalizeToolSummary(items)
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "上一轮工具操作摘要：")
	for _, item := range items {
		lines = append(lines, "- "+item.Name+" ["+item.Status+"]: "+item.Summary)
	}
	return strings.Join(lines, "\n")
}
