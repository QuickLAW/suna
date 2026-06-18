package chat

import (
	"strings"
	"testing"
)

func TestViewPlacesInputSeparatorOnOwnLineWhenContentHasNoTrailingNewline(t *testing.T) {
	var m Model
	m.InitComponents(ComponentDeps{})
	got := m.View(ViewDeps{
		Width:          40,
		MiniPet:        "pet\npet\npet",
		TopMeta:        "model",
		Conn:           "ok",
		Separator:      "----------",
		InputSeparator: "==========",
		Content:        "last content line",
		InputArea:      "input",
		StatusBar:      "status",
	})
	if !strings.Contains(got, "last content line\n==========\ninput") {
		t.Fatalf("input separator should be on its own line directly above input, got:\n%s", got)
	}
}
