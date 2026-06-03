package tui

import "testing"

func TestResolveThemeUsesExpectedTheme(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "auto defaults to light", input: ThemeAuto, want: ThemeLight},
		{name: "explicit dark", input: ThemeDark, want: ThemeDark},
	}

	t.Setenv("COLORFGBG", "")
	t.Setenv("TERMINAL_THEME", "")
	t.Setenv("THEME", "")
	t.Setenv("APPEARANCE", "")

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTheme(tt.input).Name; got != tt.want {
				t.Fatalf("resolveTheme(%q).Name = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
