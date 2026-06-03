package model

import (
	"io"
	"strings"
	"testing"
)

type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

func TestCompatibleSSEDecoderSkipsHeartbeat(t *testing.T) {
	decoder := newCompatibleSSEDecoder(nopReadCloser{strings.NewReader(": ping\n\ndata: {\"ok\":true}\n\n")})
	if !decoder.Next() {
		t.Fatalf("Next() = false, err = %v", decoder.Err())
	}
	if got := strings.TrimSpace(string(decoder.Event().Data)); got != `{"ok":true}` {
		t.Fatalf("Event().Data = %q, want %q", got, `{"ok":true}`)
	}
	if got := decoder.Next(); got {
		t.Fatalf("Next() = %v, want false after last event", got)
	}
}

func TestCompatibleSSEDecoderSkipsEmptyData(t *testing.T) {
	decoder := newCompatibleSSEDecoder(nopReadCloser{strings.NewReader("data: \n\ndata: [DONE]\n\n")})
	if !decoder.Next() {
		t.Fatalf("Next() = false, err = %v", decoder.Err())
	}
	if got := strings.TrimSpace(string(decoder.Event().Data)); got != "[DONE]" {
		t.Fatalf("Event().Data = %q, want %q", got, "[DONE]")
	}
}
