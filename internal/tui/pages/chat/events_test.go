package chat

import (
	"testing"
	"time"

	"github.com/alanchenchen/suna/internal/protocol"
	"github.com/alanchenchen/suna/internal/tui/components/toolview"
)

func TestEndToolUpdatesTranscriptEntryAfterAskClearsActiveTools(t *testing.T) {
	started := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	ended := started.Add(2 * time.Second)
	entry := &toolview.Entry{ID: "ask-tool", Status: toolview.StatusRunning, StartedAt: started}
	block := &toolview.Block{Entries: map[string]*toolview.Entry{"ask-tool": entry}, Order: []string{"ask-tool"}}
	m := Model{
		Messages:       []Msg{{Role: "tool", Content: block}},
		ActiveTools:    map[string]*toolview.Entry{},
		ToolStartTimes: map[string]time.Time{},
	}

	m.EndTool(protocol.ToolEndParams{ID: "ask-tool", Result: `{"answer":"ok"}`}, "ask-tool", ended)

	if entry.Status != toolview.StatusDone {
		t.Fatalf("entry.Status = %v, want done", entry.Status)
	}
	if entry.Result != `{"answer":"ok"}` {
		t.Fatalf("entry.Result = %q", entry.Result)
	}
	if entry.EndedAt != ended {
		t.Fatalf("entry.EndedAt = %v, want %v", entry.EndedAt, ended)
	}
	if entry.Duration != 2*time.Second {
		t.Fatalf("entry.Duration = %v, want 2s", entry.Duration)
	}
	if _, ok := m.ActiveTools["ask-tool"]; ok {
		t.Fatalf("ActiveTools still contains ended tool")
	}
}
