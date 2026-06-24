package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmUpdateAcceptsOnlyExplicitYes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "yes short", input: "y\n", want: true},
		{name: "yes long", input: "YES\n", want: true},
		{name: "default no", input: "\n", want: false},
		{name: "no", input: "n\n", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if got := confirmUpdate(strings.NewReader(tt.input), &out); got != tt.want {
				t.Fatalf("confirmUpdate() = %v, want %v", got, tt.want)
			}
			if !strings.Contains(out.String(), "[y/N]") {
				t.Fatalf("prompt = %q, want confirmation marker", out.String())
			}
		})
	}
}

func TestLimitReleaseNotesTruncatesByRunes(t *testing.T) {
	got := limitReleaseNotes("你好世界", 2)
	want := "你好\n..."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
