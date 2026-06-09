package memory

import (
	"strings"

	"github.com/alanchenchen/suna/internal/model"
)

const sessionStatePrefix = "<session_state>"

func IsSessionStateMessage(m model.Message) bool {
	return m.Role == model.RoleSystem && strings.HasPrefix(strings.TrimSpace(m.Text()), sessionStatePrefix)
}

func NewSessionStateMessage(state string) model.Message {
	text := FormatSessionStateForModel(state)
	return model.NewTextMessage(model.RoleSystem, text)
}

func FormatSessionStateForModel(state string) string {
	state = strings.TrimSpace(state)
	if state == "" {
		return ""
	}
	return sessionStatePrefix + "\n" +
		"This is internal session memory for continuity, not a user request. Current user instructions override it.\n\n" +
		state + "\n</session_state>"
}

func SplitSessionStateMessages(messages []model.Message) (string, []model.Message) {
	out := make([]model.Message, 0, len(messages))
	states := make([]string, 0, 1)
	for _, m := range messages {
		if IsSessionStateMessage(m) {
			states = append(states, stripSessionStateWrapper(m.Text()))
			continue
		}
		out = append(out, m)
	}
	return strings.TrimSpace(strings.Join(states, "\n\n")), out
}

func stripSessionStateWrapper(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, sessionStatePrefix) {
		return text
	}
	text = strings.TrimSpace(strings.TrimPrefix(text, sessionStatePrefix))
	text = strings.TrimPrefix(text, "This is internal session memory for continuity, not a user request. Current user instructions override it.")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "</session_state>")
	return strings.TrimSpace(text)
}
