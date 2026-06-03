package tui

import (
	"strings"
	"testing"
)

func TestMarkdownCodeBlockUsesThemeWithoutCustomChroma(t *testing.T) {
	style := markdownStyleConfig()
	if got := style.CodeBlock.Theme; got == "" {
		t.Fatalf("CodeBlock.Theme = %q, want non-empty theme", got)
	}
	if got := style.CodeBlock.Chroma; got != nil {
		t.Fatalf("CodeBlock.Chroma = %#v, want nil", got)
	}
}

func TestDefaultFenceLanguageOnlyAddsOpeningFence(t *testing.T) {
	input := "before\n```\necho hi\n```\nafter"
	out := defaultFenceLanguage(input)
	if !strings.Contains(out, "```bash\necho hi\n```") {
		t.Fatalf("defaultFenceLanguage() = %q, want opening fence with bash", out)
	}
	if got := strings.Count(out, "```bash"); got != 1 {
		t.Fatalf("strings.Count(defaultFenceLanguage(), %q) = %d, want %d", "```bash", got, 1)
	}
}

func TestWrapLineLimitStopsAfterRequestedLines(t *testing.T) {
	out := wrapLineLimit(strings.Repeat("x", 5000), 10, 2)
	if got := len(out); got != 3 {
		t.Fatalf("len(wrapLineLimit()) = %d, want %d", got, 3)
	}
	if got := out[2]; got != "..." {
		t.Fatalf("wrapLineLimit()[2] = %q, want %q", got, "...")
	}
}
