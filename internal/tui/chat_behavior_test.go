package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alanchenchen/suna/internal/protocol"
)

func TestThinkingBoxCollapsedWhileStreamingAndStopsElapsed(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 100}
	started := time.Now().Add(-2 * time.Second)
	ended := started.Add(1500 * time.Millisecond)

	streaming := stripANSIForTest(tui.renderThinkingBox("第一段\n第二段\n最终判断", true, started, time.Time{}))
	if strings.Contains(streaming, "第一段") || strings.Contains(streaming, "第二段") {
		t.Fatalf("renderThinkingBox(streaming) = %q, should not show hidden reasoning lines", streaming)
	}
	if !strings.Contains(streaming, "最终判断") || !strings.Contains(streaming, "Ctrl+R") {
		t.Fatalf("renderThinkingBox(streaming) = %q, want compact summary and Ctrl+R hint", streaming)
	}

	completed := stripANSIForTest(tui.renderThinkingBox("第一段\n第二段\n最终判断", false, started, ended))
	if !strings.Contains(completed, "1.5s") {
		t.Fatalf("renderThinkingBox(completed) = %q, want fixed duration", completed)
	}
	if strings.Contains(completed, "第一段") || strings.Contains(completed, "第二段") {
		t.Fatalf("renderThinkingBox(completed) = %q, should not show hidden reasoning lines", completed)
	}
}

func TestSendingMessageForcesScrollToBottom(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 18}
	tui.initChatComponents()
	for i := 0; i < 40; i++ {
		tui.appendNonToolMessage(chatMsg{role: "system", content: "历史消息"})
	}
	tui.syncContent()
	tui.vp.SetYOffset(0)
	tui.followBottom = false
	tui.ta.SetValue("新的问题")

	tui.handleSend()
	if !tui.vp.AtBottom() {
		t.Fatalf("vp.AtBottom() = false after message send; YOffset = %d", tui.vp.YOffset())
	}
	if !tui.followBottom {
		t.Fatalf("followBottom = false after message send, want true")
	}
}

func TestSlashCommandForcesScrollToBottom(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 18}
	tui.initChatComponents()
	for i := 0; i < 40; i++ {
		tui.appendNonToolMessage(chatMsg{role: "system", content: "历史消息"})
	}
	tui.syncContent()
	tui.vp.SetYOffset(0)
	tui.followBottom = false
	tui.ta.SetValue("/compact")

	tui.handleSend()
	if !tui.vp.AtBottom() {
		t.Fatalf("vp.AtBottom() = false after slash command; YOffset = %d", tui.vp.YOffset())
	}
	if !tui.followBottom {
		t.Fatalf("followBottom = false after slash command, want true")
	}
}

func TestActiveReasoningSuppressesDuplicateStatusLine(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24}
	tui.initChatComponents()
	tui.loading = true
	tui.phase = phaseThinking
	tui.phaseStart = time.Now().Add(-time.Second)
	tui.appendNonToolMessage(chatMsg{role: "reasoning", content: "正在分析", streaming: true, startedAt: time.Now().Add(-time.Second)})

	tui.syncContent()
	view := stripANSIForTest(tui.vp.View())
	if count := strings.Count(view, "◎ 思考"); count != 1 {
		t.Fatalf("strings.Count(view, %q) = %d, want %d; view = %q", "◎ 思考", count, 1, view)
	}
	if strings.Contains(view, "Esc 取消") {
		t.Fatalf("view = %q, should not contain duplicate bottom status line", view)
	}
}

func TestWaitingWithoutVisibleProgressShowsStatusLine(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24}
	tui.initChatComponents()
	tui.loading = true
	tui.phase = phaseFirstLLM
	tui.phaseStart = time.Now().Add(-time.Second)

	tui.syncContent()
	view := stripANSIForTest(tui.vp.View())
	if !strings.Contains(view, "等待 LLM") || !strings.Contains(view, "Esc 取消") {
		t.Fatalf("view = %q, want cancellable wait status line", view)
	}
}

func TestRunningToolSuppressesDuplicateStatusLine(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24}
	tui.initChatComponents()
	tui.loading = true
	tui.phase = phaseTool
	tui.phaseStart = time.Now().Add(-time.Second)
	block := tui.ensureToolBlock()
	block.add(&toolEntry{id: "1", name: "Read", intent: "读取文件", status: toolRunning, startedAt: time.Now().Add(-time.Second)})
	tui.activeTools = map[string]*toolEntry{"1": block.entries["1"]}

	tui.syncContent()
	view := stripANSIForTest(tui.vp.View())
	if strings.Contains(view, "Esc 取消") {
		t.Fatalf("view = %q, should not contain duplicate bottom status line", view)
	}
}

func TestLockedInputShowsStatusPlaceholder(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24}
	tui.initChatComponents()
	tui.loading = true
	tui.phase = phaseLLM
	tui.phaseStart = time.Now()
	tui.ta.Blur()

	view := stripANSIForTest(tui.renderInputArea())
	if !strings.Contains(view, "正在回复") || !strings.Contains(view, "Esc") {
		t.Fatalf("renderInputArea() = %q, want active status and cancel hint", view)
	}
	if tui.ta.Focused() {
		t.Fatalf("textarea.Focused() = true while input is locked, want false")
	}
}

func TestWelcomeNewInitializesChatBeforeResetPhase(t *testing.T) {
	tui := &TUI{i18n: newTranslator(LocaleZH), width: 80, height: 24, ready: true}
	tui.configState = protocol.ConfigParams{Models: []protocol.ConfigModel{{Provider: "test", Model: "model"}}}
	tui.initWelcomeList()

	_, cmd := tui.updateWelcome(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if tui.mode != "chat" {
		t.Fatalf("mode = %q, want %q", tui.mode, "chat")
	}
	if tui.ta.Placeholder == "" {
		t.Fatalf("textarea.Placeholder = empty, want initialized chat textarea")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want chat focus command")
	}
}
