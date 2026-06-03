package tui

import (
	"testing"

	"github.com/alanchenchen/suna/internal/protocol"
)

func TestGPTReasoningUsesResponsesForOpenAI(t *testing.T) {
	tui := &TUI{configDetailRef: "openai/gpt-5", configState: testReasoningConfig("openai", "gpt-5")}
	got := tui.gptReasoning("high")
	reasoning, ok := got["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("gptReasoning(high)[reasoning] = %#v, want map", got["reasoning"])
	}
	if got := reasoning["effort"]; got != "high" {
		t.Fatalf("gptReasoning(high)[reasoning][effort] = %#v, want %q", got, "high")
	}
}

func TestGPTReasoningUsesChatForCompatible(t *testing.T) {
	tui := &TUI{configDetailRef: "deepseek/deepseek-v4-pro", configState: testReasoningConfig("deepseek", "deepseek-v4-pro")}
	got := tui.gptReasoning("none")
	if got["reasoning_effort"] != "none" {
		t.Fatalf("gptReasoning(none)[reasoning_effort] = %#v, want %q", got["reasoning_effort"], "none")
	}
}

func TestReasoningLabelMatch(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleEN), configDetailRef: "deepseek/deepseek-v4-pro", configState: testReasoningConfig("deepseek", "deepseek-v4-pro")}
	mc := tui.configModelsSnapshot()[0]
	mc.Reasoning = deepSeekReasoning("max")
	if got := tui.reasoningDisplay(mc); got != "DeepSeek V4 / Max" {
		t.Fatalf("reasoningDisplay() = %q, want %q", got, "DeepSeek V4 / Max")
	}
}

func TestSaveReasoningUpdatesDetailStateImmediately(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleEN), configDetailRef: "deepseek/deepseek-v4-pro", configState: testReasoningConfig("deepseek", "deepseek-v4-pro")}

	tui.saveReasoning(deepSeekReasoning("max"))
	mc, ok := tui.modelByRef("deepseek/deepseek-v4-pro")
	if !ok {
		t.Fatalf("modelByRef(%q) ok = false, want true", "deepseek/deepseek-v4-pro")
	}
	if got := tui.reasoningDisplay(mc); got != "DeepSeek V4 / Max" {
		t.Fatalf("reasoningDisplay() after save = %q, want %q", got, "DeepSeek V4 / Max")
	}

	tui.saveReasoning(nil)
	mc, ok = tui.modelByRef("deepseek/deepseek-v4-pro")
	if !ok {
		t.Fatalf("modelByRef(%q) after clear ok = false, want true", "deepseek/deepseek-v4-pro")
	}
	if got := tui.reasoningDisplay(mc); got != "" {
		t.Fatalf("reasoningDisplay() after clear = %q, want empty", got)
	}
}

func testReasoningConfig(provider, model string) protocol.ConfigParams {
	return protocol.ConfigParams{Models: []protocol.ConfigModel{{Provider: provider, Model: model}}}
}
