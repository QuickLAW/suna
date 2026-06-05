package overlay

import "testing"

func TestOverlayBlock(t *testing.T) {
	got := OverlayBlock("a\nb\nc", "x\ny\n")
	if got != "x\ny\nc" {
		t.Fatalf("OverlayBlock() = %q", got)
	}
	got = OverlayBlock("a", "x\ny")
	if got != "x\ny" {
		t.Fatalf("OverlayBlock(extends) = %q", got)
	}
}
