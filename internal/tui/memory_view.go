package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/alanchenchen/suna/internal/protocol"
)

func (t *TUI) renderMemoryList(memories []protocol.MemoryItem) string {
	width := max(36, min(t.width-6, 92))
	inner := max(24, width-8)
	var lines []string
	lines = append(lines, styleHL.Render(t.tr("tui.memory.active_title")))
	for _, m := range memories {
		lines = append(lines, renderMemoryItem(m, inner)...)
	}
	return boxStyle.Width(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func renderMemoryItem(m protocol.MemoryItem, width int) []string {
	badge := fmt.Sprintf("%s:%d", m.Kind, m.Priority)
	if m.IsCore {
		badge = "core " + badge
	}
	head := styleTool.Render("[" + badge + "]")
	content := strings.TrimSpace(m.Content)
	if content == "" {
		content = "-"
	}
	wrapped := wrapLine(content, max(12, width-4))
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}
	lines := []string{"  " + styleDim.Render("• ") + head}
	for _, line := range wrapped {
		lines = append(lines, "    "+styleToolDim.Render(line))
	}
	return lines
}

func lipglossWidthPlain(s string) int {
	return lipgloss.Width(s)
}
