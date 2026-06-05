package chat

func (m *Model) SyncInputFocus(inputLocked bool) bool {
	if m.Textarea.Placeholder == "" {
		return false
	}
	if inputLocked {
		m.Textarea.Blur()
		return false
	}
	return true
}

func (m *Model) SetInputValue(input string, chatActive bool) bool {
	if chatActive && m.Textarea.Placeholder != "" {
		m.Textarea.SetValue(input)
		m.Textarea.CursorEnd()
		return true
	}
	m.PendingInput = input
	return false
}

func (m *Model) HasDraft() bool {
	return trimSpace(m.Textarea.Value()) != "" || len(m.Attachments) > 0
}

func (m *Model) AcceptCommandSuggestion() (CommandSpec, bool) {
	suggestion, ok := m.SelectedCommandSuggestion()
	if !ok {
		return CommandSpec{}, false
	}
	m.Textarea.Reset()
	m.ClearCommandSuggestions()
	return suggestion, true
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
