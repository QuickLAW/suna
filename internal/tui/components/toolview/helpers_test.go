package toolview

import (
	"strings"
	"testing"
)

func TestCompactPathKeepsFullPathWhenItFits(t *testing.T) {
	path := "Users/alanchen/Documents/suna/internal/runner/types.go"
	if got := CompactPath(path, 80); got != path {
		t.Fatalf("CompactPath() = %q, want full path", got)
	}
}

func TestCompactPathKeepsFilenameSuffixWhenTight(t *testing.T) {
	got := CompactPath("very/long/path/internal/runner/types.go", 12)
	if len(got) == 0 || got == "types.go" || !strings.HasPrefix(got, "…") || got[len(got)-len("types.go"):] != "types.go" {
		t.Fatalf("CompactPath() = %q, want ellipsized filename suffix", got)
	}
}
