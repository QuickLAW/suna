package tui

import (
	"testing"

	"github.com/alanchenchen/suna/internal/protocol"
)

func TestGPTReasoningUsesResponsesForOpenAI(t *testing.T) {
	tui := &TUI{configDetailRef: "openai/gpt-5", configState: testReasoningConfig("openai", "gpt-5")}
	got := tui.gptReasoning("high")
	reasoning, ok := got["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "high" {
		t.Fatalf("gpt openai reasoning = %#v", got)
	}
}

func TestGPTReasoningUsesChatForCompatible(t *testing.T) {
	tui := &TUI{configDetailRef: "deepseek/deepseek-v4-pro", configState: testReasoningConfig("deepseek", "deepseek-v4-pro")}
	got := tui.gptReasoning("none")
	if got["reasoning_effort"] != "none" {
		t.Fatalf("gpt compatible reasoning = %#v", got)
	}
}

func TestReasoningLabelMatch(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleEN), configDetailRef: "deepseek/deepseek-v4-pro", configState: testReasoningConfig("deepseek", "deepseek-v4-pro")}
	mc := tui.configModelsSnapshot()[0]
	mc.Reasoning = deepSeekReasoning("max")
	label := tui.reasoningDisplay(mc)
	if label != "DeepSeek V4 / Max" {
		t.Fatalf("label = %q", label)
	}
}

func TestSaveReasoningUpdatesDetailStateImmediately(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleEN), configDetailRef: "deepseek/deepseek-v4-pro", configState: testReasoningConfig("deepseek", "deepseek-v4-pro")}

	tui.saveReasoning(deepSeekReasoning("max"))
	mc, ok := tui.modelByRef("deepseek/deepseek-v4-pro")
	if !ok {
		t.Fatal("model not found")
	}
	if got := tui.reasoningDisplay(mc); got != "DeepSeek V4 / Max" {
		t.Fatalf("reasoningDisplay after save = %q", got)
	}

	tui.saveReasoning(nil)
	mc, ok = tui.modelByRef("deepseek/deepseek-v4-pro")
	if !ok {
		t.Fatal("model not found after clear")
	}
	if got := tui.reasoningDisplay(mc); got != "" {
		t.Fatalf("reasoningDisplay after clear = %q, want empty", got)
	}
}

func testReasoningConfig(provider, model string) protocol.ConfigParams {
	return protocol.ConfigParams{Models: []protocol.ConfigModel{{Provider: provider, Model: model}}}
}
