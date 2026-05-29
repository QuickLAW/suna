package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"

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

func TestConfigModelRefAfterEditUsesNewRef(t *testing.T) {
	tui := &TUI{
		i18n:              newTranslator(LocaleEN),
		configEditingName: "openai/gpt-4o-mini",
		configInputs:      providerInputsForTest("openai", "gpt-4o", "", "https://api.openai.com/v1", "128000", ""),
		configState: protocol.ConfigParams{ActiveModel: "openai/gpt-4o", Models: []protocol.ConfigModel{
			{Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com/v1", ContextWindow: 128000},
		}},
	}

	if got := tui.configProviderFormRef(); got != "openai/gpt-4o" {
		t.Fatalf("configProviderFormRef = %q", got)
	}
	if !tui.openConfigDetailIfPresent(tui.configProviderFormRef()) {
		t.Fatalf("openConfigDetailIfPresent returned false")
	}
	if tui.configPage != "detail" || tui.configDetailRef != "openai/gpt-4o" {
		t.Fatalf("detail page/ref = %q/%q", tui.configPage, tui.configDetailRef)
	}
}

func TestReturnToConfigModelsClearsMissingDetail(t *testing.T) {
	tui := &TUI{
		i18n:            newTranslator(LocaleEN),
		configPage:      "detail",
		configDetailRef: "openai/gpt-4o-mini",
		configState: protocol.ConfigParams{ActiveModel: "anthropic/claude-sonnet", Models: []protocol.ConfigModel{
			{Provider: "anthropic", Model: "claude-sonnet"},
		}},
	}

	if _, ok := tui.modelByRef(tui.configDetailRef); ok {
		t.Fatalf("deleted model unexpectedly exists")
	}
	tui.returnToConfigModels()
	if tui.configPage != "models" || tui.configDetailRef != "" {
		t.Fatalf("page/ref = %q/%q, want models/empty", tui.configPage, tui.configDetailRef)
	}
	rows := tui.configModelRows()
	if tui.configCursor < 0 || tui.configCursor >= len(rows) || rows[tui.configCursor].name != "anthropic/claude-sonnet" {
		t.Fatalf("cursor = %d rows = %#v", tui.configCursor, rows)
	}
}

func providerInputsForTest(values ...string) []textinput.Model {
	inputs := make([]textinput.Model, len(values))
	for i, value := range values {
		in := textinput.New()
		in.SetValue(value)
		inputs[i] = in
	}
	return inputs
}
