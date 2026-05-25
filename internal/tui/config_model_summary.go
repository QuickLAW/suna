package tui

import "strings"

func modelSummary(mc tuiModelConfig, active bool) string {
	var parts []string
	if active {
		parts = append(parts, "active")
	}
	if !mc.HasAPIKey {
		parts = append(parts, "missing_api_key")
	} else if mc.Ref() == "" {
		parts = append(parts, "invalid")
	}
	parts = append(parts, mc.Provider, mc.Model)
	if mc.ContextWindow > 0 {
		parts = append(parts, "ctx "+fmtTok(mc.ContextWindow))
	}
	if mc.BaseURL != "" {
		parts = append(parts, "endpoint_configured")
	}
	if len(mc.Strengths) > 0 {
		parts = append(parts, strings.Join(mc.Strengths, ", "))
	}
	return strings.Join(parts, " · ")
}

func (t *TUI) modelSummary(mc tuiModelConfig) string {
	raw := modelSummary(mc, t.isActiveModelRef(mc.Ref()))
	parts := strings.Split(raw, " · ")
	for i, part := range parts {
		switch part {
		case "active":
			parts[i] = t.tr("tui.config.activated_status")
		case "missing_api_key":
			parts[i] = t.tr("tui.config.missing_api_key")
		case "invalid":
			parts[i] = t.tr("tui.config.invalid")
		case "endpoint_configured":
			parts[i] = t.tr("tui.config.endpoint_configured")
		}
	}
	return strings.Join(parts, " · ")
}
