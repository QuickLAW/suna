package tui

import "testing"

func TestResolveThemeAutoDefaultsToLight(t *testing.T) {
	t.Setenv("COLORFGBG", "")
	t.Setenv("TERMINAL_THEME", "")
	t.Setenv("THEME", "")
	t.Setenv("APPEARANCE", "")

	if got := resolveTheme(ThemeAuto).Name; got != ThemeLight {
		t.Fatalf("auto theme = %q, want light", got)
	}
}

func TestResolveThemeExplicitDark(t *testing.T) {
	if got := resolveTheme(ThemeDark).Name; got != ThemeDark {
		t.Fatalf("dark theme = %q, want dark", got)
	}
}
