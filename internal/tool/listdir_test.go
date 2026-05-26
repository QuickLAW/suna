package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirPaginatesLargeDirectory(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 25; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%02d.txt", i))
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	res := ListDir{}.Execute(context.Background(), map[string]any{"path": dir, "offset": float64(11), "limit": float64(5)})
	if res.IsError {
		t.Fatalf("listdir error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "file-11.txt") || !strings.Contains(res.Content, "file-15.txt") {
		t.Fatalf("expected paged entries, got:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "file-10.txt") || strings.Contains(res.Content, "file-16.txt") {
		t.Fatalf("unexpected entries outside page, got:\n%s", res.Content)
	}
	if !res.Truncated || !strings.Contains(res.Content, "Use offset=16") {
		t.Fatalf("expected continuation hint, got:\n%s", res.Content)
	}
}
