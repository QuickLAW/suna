package tui

import (
	"testing"

	"github.com/alanchenchen/suna/internal/protocol"
)

func TestConfigDeleteOptionsOfferAPIKeyForLastProviderModel(t *testing.T) {
	tui := &TUI{
		i18n:                newTranslator(LocaleEN),
		configDeleteConfirm: "openai/gpt-4o-mini",
		configState: protocol.ConfigParams{Models: []protocol.ConfigModel{
			{Provider: "openai", Model: "gpt-4o-mini", HasAPIKey: true},
			{Provider: "anthropic", Model: "claude-sonnet", HasAPIKey: true},
		}},
	}

	if !tui.shouldOfferDeleteAPIKey("openai/gpt-4o-mini") {
		t.Fatalf("shouldOfferDeleteAPIKey = false, want true")
	}
	options := tui.configDeleteOptions()
	if len(options) != 3 {
		t.Fatalf("len(options) = %d, want 3: %#v", len(options), options)
	}
}

func TestConfigDeleteOptionsHideAPIKeyWhenProviderStillUsed(t *testing.T) {
	tui := &TUI{
		i18n:                newTranslator(LocaleEN),
		configDeleteConfirm: "openai/gpt-4o-mini",
		configState: protocol.ConfigParams{Models: []protocol.ConfigModel{
			{Provider: "openai", Model: "gpt-4o-mini", HasAPIKey: true},
			{Provider: "openai", Model: "gpt-4o", HasAPIKey: true},
		}},
	}

	if tui.shouldOfferDeleteAPIKey("openai/gpt-4o-mini") {
		t.Fatalf("shouldOfferDeleteAPIKey = true, want false")
	}
	options := tui.configDeleteOptions()
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2: %#v", len(options), options)
	}
}

func TestConfigDeleteOptionsHideAPIKeyWhenMissing(t *testing.T) {
	tui := &TUI{
		i18n:                newTranslator(LocaleEN),
		configDeleteConfirm: "openai/gpt-4o-mini",
		configState: protocol.ConfigParams{Models: []protocol.ConfigModel{
			{Provider: "openai", Model: "gpt-4o-mini"},
		}},
	}

	if tui.shouldOfferDeleteAPIKey("openai/gpt-4o-mini") {
		t.Fatalf("shouldOfferDeleteAPIKey = true, want false")
	}
	options := tui.configDeleteOptions()
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2: %#v", len(options), options)
	}
}

func TestSwitchModelRefUpdatesActiveProviderModel(t *testing.T) {
	tui := &TUI{
		i18n:         newTranslator(LocaleEN),
		providerName: "openai",
		modelName:    "gpt-4o-mini",
		configState: protocol.ConfigParams{ActiveModel: "openai/gpt-4o-mini", Models: []protocol.ConfigModel{
			{Provider: "openai", Model: "gpt-4o-mini", ContextWindow: 128000},
			{Provider: "anthropic", Model: "claude-sonnet", ContextWindow: 200000},
		}},
	}

	cmd := tui.switchModelRef("anthropic/claude-sonnet")
	if cmd == nil {
		t.Fatalf("switchModelRef returned nil cmd")
	}
	if tui.configState.ActiveModel != "anthropic/claude-sonnet" {
		t.Fatalf("ActiveModel = %q", tui.configState.ActiveModel)
	}
	if tui.providerName != "anthropic" || tui.modelName != "claude-sonnet" {
		t.Fatalf("provider/model = %q/%q", tui.providerName, tui.modelName)
	}
	if tui.contextWindow != 200000 {
		t.Fatalf("contextWindow = %d", tui.contextWindow)
	}
}
