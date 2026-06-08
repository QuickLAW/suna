package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/alanchenchen/suna/internal/model"
	"github.com/alanchenchen/suna/internal/prompt"
)

type captureCompressProvider struct {
	request *model.CompletionRequest
	text    string
}

func (p *captureCompressProvider) Complete(ctx context.Context, req *model.CompletionRequest) (<-chan model.Chunk, error) {
	p.request = req
	ch := make(chan model.Chunk, 1)
	ch <- model.Chunk{Content: p.text, Done: true}
	close(ch)
	return ch, nil
}
func (p *captureCompressProvider) EstimateTokens(text string) int { return len(text) / 4 }
func (p *captureCompressProvider) ContextWindow() int             { return 100000 }

func TestCompressHistoryBuildsContinuationState(t *testing.T) {
	provider := &captureCompressProvider{text: "# Continuation State\n\n## User goal\n- continue task"}
	compressor := NewCompressor(provider)
	loader, err := prompt.New()
	if err != nil {
		t.Fatal(err)
	}
	compressor.SetPrompts(loader)

	messages := []model.Message{
		model.NewTextMessage(model.RoleUser, "我要一个最小改动方案，同时记住之前讨论的约束。"+strings.Repeat("补充背景。", 80)),
		model.NewTextMessage(model.RoleAssistant, "可以新增很多协议字段。"),
		model.NewTextMessage(model.RoleUser, "不要新增复杂语义，复用现有 compact。"),
		model.NewTextMessage(model.RoleAssistant, "好的，改为 continuation state。"),
	}
	compressed, summary, err := compressor.CompressHistoryKeeping(context.Background(), messages, 1)
	if err != nil {
		t.Fatal(err)
	}
	if summary != provider.text {
		t.Fatalf("summary = %q, want provider text", summary)
	}
	if len(compressed) != 2 {
		t.Fatalf("len(compressed) = %d, want 2", len(compressed))
	}
	first := compressed[0].Text()
	if !strings.HasPrefix(first, "Continuation state for the ongoing task:") {
		t.Fatalf("first message = %q, want continuation state prefix", first)
	}
	input := provider.request.Messages[0].Text()
	for _, want := range []string{"# Continuation State", "## User constraints / preferences", "## Rejected directions", "<user_message", "不要新增复杂语义"} {
		if !strings.Contains(input, want) {
			t.Fatalf("compress input missing %q:\n%s", want, input)
		}
	}
}

func TestFormatCompressInputDenoisesToolOutput(t *testing.T) {
	longToolOutput := strings.Repeat("tool-log-line\n", 800)
	messages := []model.Message{
		model.NewTextMessage(model.RoleUser, "请检查压缩策略，不要只按 coding 优化。"),
		{
			Role:      model.RoleAssistant,
			ToolCalls: []model.ToolCall{{ID: "call-1", Name: "readfile", Arguments: `{"path":"internal/memory/compress.go","content":"` + strings.Repeat("x", 6000) + `"}`}},
		},
		{Role: model.RoleTool, ToolCallID: "call-1", TextContent: longToolOutput, Content: []model.ContentBlock{{Type: model.ContentText, Text: longToolOutput}}},
	}

	input := formatCompressInput(messages)
	for _, want := range []string{"<user_message", "不要只按 coding 优化", "<tool_call", `name="readfile"`, "<tool_result", "truncated"} {
		if !strings.Contains(input, want) {
			t.Fatalf("formatted input missing %q:\n%s", want, input)
		}
	}
	if strings.Contains(input, strings.Repeat("tool-log-line\n", 500)) {
		t.Fatalf("formatted input kept too much raw tool output")
	}
	if strings.Contains(input, strings.Repeat("x", 5000)) {
		t.Fatalf("formatted input kept too much raw tool arguments")
	}
}
