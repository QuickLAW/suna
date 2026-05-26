package tool

import (
	"context"
	"strings"
	"testing"
)

func TestExecLimitsLargeStdout(t *testing.T) {
	res := Exec{}.Execute(context.Background(), map[string]any{
		"command": "yes x | head -c 200000",
		"timeout": float64(5),
		"shell":   "bash",
	})
	if res.IsError {
		t.Fatalf("exec error: %s", res.Error)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated output")
	}
	if len(res.Content) > maxExecOutput+100 {
		t.Fatalf("output too large: %d", len(res.Content))
	}
	if !strings.Contains(res.Content, "truncated") {
		start := len(res.Content) - 80
		if start < 0 {
			start = 0
		}
		t.Fatalf("expected truncation marker, got: %q", res.Content[start:])
	}
}
