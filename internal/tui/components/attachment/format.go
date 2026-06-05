package attachment

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

func FormatSize(n int64) string {
	if n <= 0 {
		return "-"
	}
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
}

func TruncateMiddle(s string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return s
	}
	r := []rune(s)
	if len(r) <= maxWidth || maxWidth <= 3 {
		return string(r[:min(len(r), maxWidth)])
	}
	keep := maxWidth - 1
	left := keep / 2
	right := keep - left
	return string(r[:left]) + "…" + string(r[len(r)-right:])
}
