package config

import (
	"strings"
	"testing"
)

func TestModelRowsActiveModelUsesMarkerWithoutRepeatedActiveText(t *testing.T) {
	m := &Model{Page: "models"}
	rows := m.ModelRows(RowsDeps{
		Tr: func(key string) string {
			if key == "tui.config.activated_status" {
				return "Activated"
			}
			return key
		},
		Models:   []ModelConfig{{Provider: "openai", Model: "gpt-4o", BaseURL: "https://example.test", ContextWindow: 128000, MaxOutputTokens: 8192, HasAPIKey: true}},
		IsActive: func(ref string) bool { return ref == "openai/gpt-4o" },
		ModelSummary: func(ModelConfig) string {
			return "openai · gpt-4o"
		},
	})

	if len(rows) == 0 {
		t.Fatalf("ModelRows returned no rows")
	}
	label := rows[0].Label
	value := rows[0].Value
	if !strings.HasPrefix(label, "◉ ") {
		t.Fatalf("active model label = %q, want active marker prefix", label)
	}
	if strings.Contains(label, "Activated") || strings.Contains(value, "Activated") {
		t.Fatalf("active model row = %q / %q, should not repeat active text", label, value)
	}
}

func TestModelSummaryKeepsCapabilitiesBriefAndPrioritizesStrengths(t *testing.T) {
	mc := ModelConfig{
		Provider:        "DF",
		Model:           "MiniMax-M3",
		BaseURL:         "https://example.test",
		ContextWindow:   1000000,
		MaxOutputTokens: 128000,
		Strengths:       []string{"多模态", "1M长上下文", "快速代码辅助"},
		HasAPIKey:       true,
	}

	got := ModelSummary(mc, true, func(n int) string {
		switch n {
		case 1000000:
			return "1.0M"
		case 128000:
			return "128.0k"
		default:
			return "?"
		}
	})
	want := "ctx 1.0M · out 128.0k · 多模态, 1M长上下文, 快速代码辅助"
	if got != want {
		t.Fatalf("ModelSummary() = %q, want %q", got, want)
	}
	for _, unexpected := range []string{"DF", "MiniMax-M3", "endpoint_configured", "active"} {
		if strings.Contains(got, unexpected) {
			t.Fatalf("ModelSummary() = %q, should not contain %q", got, unexpected)
		}
	}
}
