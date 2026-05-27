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
