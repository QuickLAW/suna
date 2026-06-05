package text

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func IndentLines(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func IndentWrappedPlain(s, prefix string, width int) string {
	if s == "" {
		return prefix
	}
	var out []string
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		for _, wrapped := range WrapLine(line, width) {
			out = append(out, prefix+wrapped)
		}
	}
	return strings.Join(out, "\n")
}

func TruncateRunes(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+3 > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

func WrapLine(s string, maxWidth int) []string {
	return WrapLineLimit(s, maxWidth, 0)
}

func WrapLineLimit(s string, maxWidth int, maxLines int) []string {
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return []string{s}
	}
	wrappedText := ansi.GraphemeWidth.Hardwrap(s, maxWidth, true)
	lines := strings.Split(wrappedText, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		return append(append([]string(nil), lines[:maxLines]...), "...")
	}
	return lines
}
