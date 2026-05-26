package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileStreamsPastInitialByteWindow(t *testing.T) {
	path := writeTempFile(t, buildLines(7000, 40))
	res := ReadFile{}.Execute(context.Background(), map[string]any{"path": path, "offset": float64(6500), "limit": float64(3)})
	if res.IsError {
		t.Fatalf("readfile error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "6500: line-6500") {
		t.Fatalf("expected line 6500, got:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "1: line-0001") {
		t.Fatalf("unexpected first line in paged result:\n%s", res.Content)
	}
}

func TestReadFileTruncatesByResultBytesWithNextOffset(t *testing.T) {
	path := writeTempFile(t, buildLines(1000, 300))
	res := ReadFile{}.Execute(context.Background(), map[string]any{"path": path, "offset": float64(1), "limit": float64(1000)})
	if res.IsError {
		t.Fatalf("readfile error: %s", res.Error)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated result")
	}
	if len(res.Content) > maxReadResultBytes+200 {
		t.Fatalf("result too large: %d", len(res.Content))
	}
	if !strings.Contains(res.Content, "Use offset=") {
		t.Fatalf("expected continuation hint, got:\n%s", res.Content)
	}
}

func TestReadFileTruncatesVeryLongLine(t *testing.T) {
	path := writeTempFile(t, strings.Repeat("x", maxReadLineBytes*2)+"\nsecond\n")
	res := ReadFile{}.Execute(context.Background(), map[string]any{"path": path, "offset": float64(1), "limit": float64(2)})
	if res.IsError {
		t.Fatalf("readfile error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "line truncated") {
		t.Fatalf("expected line truncation marker, got len=%d", len(res.Content))
	}
	if !strings.Contains(res.Content, "2: second") {
		t.Fatalf("expected second line after long line, got:\n%s", res.Content)
	}
}

func buildLines(count, payloadLen int) string {
	var b strings.Builder
	payload := strings.Repeat("x", payloadLen)
	for i := 1; i <= count; i++ {
		b.WriteString(fmt.Sprintf("line-%04d %s\n", i, payload))
	}
	return b.String()
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
