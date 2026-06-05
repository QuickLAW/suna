package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAskUserAllowCustomKeepsTextareaEditable(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24, ready: true}
	tui.initChatComponents()
	tui.chat.PendingAskID = "ask-custom"
	tui.chat.PendingAskCustom = true
	tui.chat.Loading = false

	if tui.inputLocked() {
		t.Fatalf("inputLocked() = true for allow_custom ask, want false")
	}

	tui.updateChat(tea.KeyPressMsg(tea.Key{Text: "自", Code: '自'}))
	if got := tui.chat.Textarea.Value(); got != "自" {
		t.Fatalf("textarea.Value() = %q, want custom ask input", got)
	}
}

func TestAskUserChoiceOnlyLocksTextareaButAllowsSelection(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24, ready: true}
	tui.initChatComponents()
	tui.chat.PendingAskID = "ask-choice"
	tui.chat.PendingAskCustom = false
	tui.chat.PendingAskOptions = []string{"A", "B"}
	tui.chat.PendingAskCursor = 0

	if !tui.inputLocked() {
		t.Fatalf("inputLocked() = false for choice-only ask, want true")
	}

	tui.updateChat(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	if got := tui.chat.Textarea.Value(); got != "" {
		t.Fatalf("textarea.Value() = %q, want choice-only ask to ignore printable input", got)
	}

	tui.updateChat(tea.KeyPressMsg(tea.Key{Text: "", Code: tea.KeyDown}))
	if got := tui.chat.PendingAskCursor; got != 1 {
		t.Fatalf("PendingAskCursor = %d, want 1 after down", got)
	}
}
