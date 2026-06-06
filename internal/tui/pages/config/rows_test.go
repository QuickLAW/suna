package config

import (
	"strings"
	"testing"
)

func TestModelRowsActiveModelIsProminent(t *testing.T) {
	m := &Model{Page: "models"}
	rows := m.ModelRows(RowsDeps{
		Tr: func(key string) string {
			if key == "tui.config.activated_status" {
				return "Activated"
			}
			return key
		},
		Models:   []ModelConfig{{Provider: "openai", Model: "gpt-4o", BaseURL: "https://example.test", HasAPIKey: true}},
		IsActive: func(ref string) bool { return ref == "openai/gpt-4o" },
		ModelSummary: func(ModelConfig) string {
			return "Activated · openai · gpt-4o"
		},
	})

	if len(rows) == 0 {
		t.Fatalf("ModelRows returned no rows")
	}
	label := rows[0].Label
	if !strings.Contains(label, "●") || !strings.Contains(label, "Activated") {
		t.Fatalf("active model label = %q, want prominent active marker", label)
	}
}
