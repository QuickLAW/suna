package chat

import "testing"

func TestResetToolStateClearsSubtaskManualSelection(t *testing.T) {
	var m Model
	m.InitComponents(ComponentDeps{})
	m.SubtaskCursor = 2
	m.SubtaskCursorUserSet = true
	m.SubtaskToolCursor = 3
	m.SubtaskToolCursorUserSet = true
	m.SubtaskToolDetailExpanded = true
	m.SubtaskToolDetailScroll = 5

	m.ResetToolState()

	if m.SubtaskCursor != 0 || m.SubtaskCursorUserSet {
		t.Fatalf("SubtaskCursor/UserSet = %d/%v, want 0/false", m.SubtaskCursor, m.SubtaskCursorUserSet)
	}
	if m.SubtaskToolCursor != 0 || m.SubtaskToolCursorUserSet {
		t.Fatalf("SubtaskToolCursor/UserSet = %d/%v, want 0/false", m.SubtaskToolCursor, m.SubtaskToolCursorUserSet)
	}
	if m.SubtaskToolDetailExpanded || m.SubtaskToolDetailScroll != 0 {
		t.Fatalf("detail expanded/scroll = %v/%d, want false/0", m.SubtaskToolDetailExpanded, m.SubtaskToolDetailScroll)
	}
}

func TestAppendMessageClearsSubtaskManualSelection(t *testing.T) {
	var m Model
	m.InitComponents(ComponentDeps{})
	m.SubtaskCursor = 2
	m.SubtaskCursorUserSet = true
	m.SubtaskToolCursor = 3
	m.SubtaskToolCursorUserSet = true
	m.SubtaskToolDetailExpanded = true
	m.SubtaskToolDetailScroll = 5

	m.AppendMessage(Msg{Role: "assistant", Content: "done"})

	if m.SubtaskCursor != 0 || m.SubtaskCursorUserSet {
		t.Fatalf("SubtaskCursor/UserSet = %d/%v, want 0/false", m.SubtaskCursor, m.SubtaskCursorUserSet)
	}
	if m.SubtaskToolCursor != 0 || m.SubtaskToolCursorUserSet {
		t.Fatalf("SubtaskToolCursor/UserSet = %d/%v, want 0/false", m.SubtaskToolCursor, m.SubtaskToolCursorUserSet)
	}
	if m.SubtaskToolDetailExpanded || m.SubtaskToolDetailScroll != 0 {
		t.Fatalf("detail expanded/scroll = %v/%d, want false/0", m.SubtaskToolDetailExpanded, m.SubtaskToolDetailScroll)
	}
}
