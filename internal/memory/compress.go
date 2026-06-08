package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/alanchenchen/suna/internal/model"
	"github.com/alanchenchen/suna/internal/prompt"
)

const (
	compressThreshold            = 0.8
	KeepRecentTurns              = 10
	maxToolOutputLines           = 500
	maxToolOutputBytes           = 50 * 1024
	maxCompressAssistantBytes    = 6 * 1024
	maxCompressToolResultBytes   = 4 * 1024
	maxCompressToolArgumentBytes = 2 * 1024
)

type Compressor struct {
	fastProvider model.Provider
	prompts      *prompt.Loader
}

func NewCompressor(fastProvider model.Provider) *Compressor {
	return &Compressor{fastProvider: fastProvider}
}

func (c *Compressor) SetPrompts(p *prompt.Loader) {
	c.prompts = p
}

func (c *Compressor) ShouldCompress(messages []model.Message, contextWindow int) bool {
	tokens := model.EstimateMessagesTokens(messages)
	return float64(tokens) > float64(contextWindow)*compressThreshold
}

// EstimateTokens 返回消息列表的估算 token 数
func (c *Compressor) EstimateTokens(messages []model.Message) int {
	return model.EstimateMessagesTokens(messages)
}

func (c *Compressor) TruncateToolOutput(content string) string {
	if len(content) <= maxToolOutputBytes {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxToolOutputLines {
		return truncateUTF8(content, maxToolOutputBytes) + "\n... (truncated)"
	}
	kept := lines[:maxToolOutputLines]
	result := strings.Join(kept, "\n")
	if len(result) > maxToolOutputBytes {
		result = truncateUTF8(result, maxToolOutputBytes)
	}
	return fmt.Sprintf("%s\n... (truncated, %d lines total)", result, len(lines))
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	for i := range s {
		if i > maxBytes {
			return s[:i]
		}
	}
	return s
}

func formatCompressInput(messages []model.Message) string {
	var sb strings.Builder
	for i, m := range messages {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatCompressMessage(i+1, m))
	}
	return sb.String()
}

func formatCompressMessage(index int, m model.Message) string {
	var sb strings.Builder
	role := string(m.Role)
	text := strings.TrimSpace(m.Text())
	switch m.Role {
	case model.RoleUser:
		if text != "" {
			sb.WriteString(fmt.Sprintf("<user_message index=%q>\n%s\n</user_message>\n", fmt.Sprint(index), text))
		}
	case model.RoleAssistant:
		if text != "" {
			sb.WriteString(fmt.Sprintf("<assistant_message index=%q note=\"assistant proposal or response; preserve only if accepted or still relevant\">\n%s\n</assistant_message>\n", fmt.Sprint(index), truncateMiddle(text, maxCompressAssistantBytes)))
		}
	case model.RoleTool:
		if text != "" {
			sb.WriteString(fmt.Sprintf("<tool_result index=%q call_id=%q note=\"convert into action facts; do not keep raw logs\">\n%s\n</tool_result>\n", fmt.Sprint(index), m.ToolCallID, truncateMiddle(text, maxCompressToolResultBytes)))
		}
	default:
		if text != "" {
			sb.WriteString(fmt.Sprintf("<%s_message index=%q>\n%s\n</%s_message>\n", role, fmt.Sprint(index), truncateMiddle(text, maxCompressAssistantBytes), role))
		}
	}
	for _, tc := range m.ToolCalls {
		sb.WriteString(fmt.Sprintf("<tool_call index=%q name=%q>\n%s\n</tool_call>\n", fmt.Sprint(index), tc.Name, truncateMiddle(tc.Arguments, maxCompressToolArgumentBytes)))
	}
	return sb.String()
}

func truncateMiddle(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	half := maxBytes / 2
	if half <= 0 {
		return "... (truncated)"
	}
	prefix := truncateUTF8(s, half)
	suffixBudget := maxBytes - len(prefix)
	if suffixBudget <= 0 {
		return prefix + "\n... (truncated)"
	}
	suffixStart := len(s) - suffixBudget
	if suffixStart < 0 {
		suffixStart = 0
	}
	for suffixStart < len(s) && !isUTF8Boundary(s[suffixStart]) {
		suffixStart++
	}
	return prefix + fmt.Sprintf("\n... (truncated, %d bytes omitted) ...\n", len(s)-len(prefix)-len(s[suffixStart:])) + s[suffixStart:]
}

func isUTF8Boundary(b byte) bool {
	return b&0xC0 != 0x80
}

func (c *Compressor) CompressHistory(ctx context.Context, messages []model.Message) ([]model.Message, string, error) {
	return c.CompressHistoryKeeping(ctx, messages, KeepRecentTurns)
}

func (c *Compressor) CompressHistoryKeeping(ctx context.Context, messages []model.Message, keepRecent int) ([]model.Message, string, error) {
	if len(messages) == 0 {
		return messages, "", nil
	}

	keep := keepRecent
	if keep <= 0 {
		keep = KeepRecentTurns
	}
	if len(messages) <= keep {
		keep = len(messages) - 1
	}
	if keep < 1 {
		keep = 1
	}

	keepStart := len(messages) - keep
	if keepStart <= 0 {
		return messages, "", nil
	}
	compressRegion := messages[:keepStart]
	keepRegion := messages[keepStart:]

	summary := formatCompressInput(compressRegion)
	if c.fastProvider != nil && len(summary) > 500 {
		compressInput := summary
		promptText := compressInput
		systemPrompt := "Compress the following conversation history into a continuation state for the ongoing task. Preserve user goals, constraints, decisions, rejected directions, current state, tool/action facts, and next steps. Convert tool output into state facts instead of logs."
		if c.prompts != nil {
			if rendered, err := c.prompts.RenderCompress(compressInput); err == nil && rendered != "" {
				systemPrompt = ""
				promptText = rendered
			}
		}
		req := &model.CompletionRequest{
			Purpose: "compress",
			Messages: []model.Message{
				model.NewTextMessage(model.RoleUser, promptText),
			},
			MaxTokens: 1000,
		}
		if systemPrompt != "" {
			req.System = systemPrompt
		}
		ch, err := c.fastProvider.Complete(ctx, req)
		if err == nil {
			var full string
			for chunk := range ch {
				if chunk.Content != "" {
					full += chunk.Content
				}
				if chunk.Done {
					break
				}
			}
			if full != "" {
				summary = full
			}
		}
	}

	stateText := "Continuation state for the ongoing task:\n" + summary
	result := []model.Message{
		{
			Role:        model.RoleSystem,
			TextContent: stateText,
			Content:     []model.ContentBlock{{Type: model.ContentText, Text: stateText}},
		},
	}
	result = append(result, keepRegion...)
	return result, summary, nil
}
