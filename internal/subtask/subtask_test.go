package subtask

import (
	"testing"

	"github.com/alanchenchen/suna/internal/model"
)

func TestToolDefsReturnsAllowedToolDefinitions(t *testing.T) {
	st := New(Request{ToolDefs: []model.ToolDef{{
		Name:        "readfile",
		Description: "read a file",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
	}}})

	defs := st.toolDefs()
	if len(defs) != 1 || defs[0].Name != "readfile" {
		t.Fatalf("toolDefs = %#v, want readfile", defs)
	}
	props := defs[0].Parameters["properties"].(map[string]any)
	props["path"] = map[string]any{"type": "number"}

	again := st.toolDefs()
	againProps := again[0].Parameters["properties"].(map[string]any)
	path := againProps["path"].(map[string]any)
	if path["type"] != "string" {
		t.Fatalf("toolDefs aliases request schema, path type = %v", path["type"])
	}
}

func TestParseFinalResultStructuredSideEffects(t *testing.T) {
	got := parseFinalResult(`{"result":"done","side_effects":{"status":"remaining","summary":"modified requested files","paths":["a.txt"]}}`)
	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", got.Status, StatusCompleted)
	}
	if got.Text != "done" {
		t.Fatalf("Text = %q, want done", got.Text)
	}
	if got.SideEffects.Status != SideEffectsRemaining {
		t.Fatalf("SideEffects.Status = %q, want %q", got.SideEffects.Status, SideEffectsRemaining)
	}
	if len(got.SideEffects.Paths) != 1 || got.SideEffects.Paths[0] != "a.txt" {
		t.Fatalf("SideEffects.Paths = %#v, want [a.txt]", got.SideEffects.Paths)
	}
}

func TestParseFinalResultUnstructuredMarksUnknown(t *testing.T) {
	got := parseFinalResult("plain answer")
	if got.Status != StatusCompletedUnstructured {
		t.Fatalf("Status = %q, want %q", got.Status, StatusCompletedUnstructured)
	}
	if got.Text != "plain answer" {
		t.Fatalf("Text = %q, want plain answer", got.Text)
	}
	if got.SideEffects.Status != SideEffectsUnknown {
		t.Fatalf("SideEffects.Status = %q, want %q", got.SideEffects.Status, SideEffectsUnknown)
	}
}

func TestParseFinalResultUnsupportedSideEffectsStatusMarksUnknown(t *testing.T) {
	got := parseFinalResult(`{"result":"done","side_effects":{"status":"maybe","summary":"custom"}}`)
	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", got.Status, StatusCompleted)
	}
	if got.SideEffects.Status != SideEffectsUnknown {
		t.Fatalf("SideEffects.Status = %q, want %q", got.SideEffects.Status, SideEffectsUnknown)
	}
	if got.SideEffects.Summary == "custom" || got.SideEffects.Summary == "" {
		t.Fatalf("SideEffects.Summary = %q, want unsupported status note plus original summary", got.SideEffects.Summary)
	}
}

func TestFailedResultUsesToolCallToChooseSideEffects(t *testing.T) {
	withoutTool := failedResult("boom", false)
	if withoutTool.Status != StatusFailed || withoutTool.SideEffects.Status != SideEffectsNone {
		t.Fatalf("without tool = %#v, want failed with none side effects", withoutTool)
	}
	withTool := failedResult("boom", true)
	if withTool.Status != StatusFailed || withTool.SideEffects.Status != SideEffectsUnknown {
		t.Fatalf("with tool = %#v, want failed with unknown side effects", withTool)
	}
}

func TestParseFinalResultAcceptsFencedJSON(t *testing.T) {
	got := parseFinalResult("```json\n{\"result\":\"done\",\"side_effects\":{\"status\":\"none\"}}\n```")
	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", got.Status, StatusCompleted)
	}
	if got.Text != "done" || got.SideEffects.Status != SideEffectsNone {
		t.Fatalf("result = %#v, want done with no side effects", got)
	}
}
