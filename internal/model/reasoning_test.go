package model

import "testing"

func TestMergeReasoningFields(t *testing.T) {
	body := map[string]any{"model": "m"}
	reasoning := map[string]any{
		"reasoning_effort": "high",
		"thinking":         map[string]any{"type": "enabled"},
	}
	if err := mergeReasoningFields(body, reasoning); err != nil {
		t.Fatalf("mergeReasoningFields error: %v", err)
	}
	if body["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort not merged: %#v", body)
	}
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking not preserved: %#v", body["thinking"])
	}
}

func TestMergeReasoningFieldsRejectsConflict(t *testing.T) {
	body := map[string]any{"model": "m"}
	if err := mergeReasoningFields(body, map[string]any{"model": "other"}); err == nil {
		t.Fatalf("mergeReasoningFields conflict succeeded, want error")
	}
}
