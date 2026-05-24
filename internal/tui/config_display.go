package tui

func contextDisplay(mc tuiModelConfig) string {
	return fmtTok(defaultContextWindow(mc))
}

func modelStatusMark(mc tuiModelConfig, active bool) string {
	if !mc.HasAPIKey || mc.Model == "" || mc.BaseURL == "" {
		return "!"
	}
	if active {
		return "◉"
	}
	return "○"
}

func (t *TUI) modelNeedsAttention(mc tuiModelConfig) bool {
	return !mc.HasAPIKey || mc.Model == "" || mc.BaseURL == ""
}
