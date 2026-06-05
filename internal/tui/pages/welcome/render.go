package welcome

import (
	"image/color"
	"strings"

	textutil "github.com/alanchenchen/suna/internal/tui/components/text"

	"charm.land/lipgloss/v2"
)

type ViewDeps struct {
	Tr          func(string) string
	Brand       lipgloss.Style
	Dim         lipgloss.Style
	HL          lipgloss.Style
	Box         lipgloss.Style
	BorderColor color.Color
}

type ViewData struct {
	Width         int
	Pet           string
	Info          string
	Menu          string
	HasConfigured bool
}

func RenderView(data ViewData, deps ViewDeps) string {
	var sb strings.Builder
	w := min(max(54, data.Width-14), 84)
	leftPad := max(2, (data.Width-w)/2)
	pad := strings.Repeat(" ", leftPad)

	sb.WriteString("\n")
	pet := strings.Split(data.Pet, "\n")
	info := strings.Split(data.Info, "\n")
	rows := max(len(pet), len(info))
	for i := 0; i < rows; i++ {
		left, right := "", ""
		if i < len(pet) {
			left = pet[i]
		}
		if i < len(info) {
			right = info[i]
		}
		sb.WriteString(pad + left + strings.Repeat(" ", max(8, 24-lipgloss.Width(left))) + right + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(pad + deps.Brand.Render("Suna") + "\n")
	sb.WriteString(pad + deps.Dim.Render(deps.Tr("tui.welcome.subtitle")) + "\n")
	if !data.HasConfigured {
		sb.WriteString("\n" + pad + deps.HL.Render(deps.Tr("tui.welcome.setup_hint")) + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(textutil.IndentLines(welcomeBoxStyle(w, deps).Render(strings.TrimRight(data.Menu, "\n")), pad) + "\n\n")
	sb.WriteString(pad + deps.Dim.Render(deps.Tr("tui.welcome.help")) + "\n")
	return sb.String()
}

func welcomeBoxStyle(width int, deps ViewDeps) lipgloss.Style {
	return deps.Box.Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(deps.BorderColor)
}
