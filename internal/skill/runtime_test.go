package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alanchenchen/suna/internal/tool"
)

func TestRuntimeManualSkillDefaultsEnabled(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "writer", "writer", "Writing.")
	rt := NewRuntime(root, &memoryStore{})

	infos, err := rt.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got := len(infos); got != 1 {
		t.Fatalf("len(infos) = %d, want %d", got, 1)
	}
	if !infos[0].Enabled || !infos[0].Valid {
		t.Fatalf("infos[0] = %#v, want enabled and valid", infos[0])
	}
	res, handled := rt.ExecuteTool(context.Background(), ToolLoad, map[string]any{"name": "writer"})
	if !handled {
		t.Fatalf("ExecuteTool(%q) handled = false, want true", ToolLoad)
	}
	if res.IsError || res.Content == "" {
		t.Fatalf("ExecuteTool(%q) result = %#v, want loaded content", ToolLoad, res)
	}
}

func TestRuntimeStartCheckExistingSkillRequiresExplicitEnable(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "report", "report", "Write reports.")
	store := &memoryStore{trust: map[string]Record{"report": {Enabled: false}}}
	prompter := &fakePrompter{answers: []string{optionReviewNo, optionEnableYes}}
	rt := newRuntimeWithPrompter(root, store, prompter)

	res, handled := rt.ExecuteTool(context.Background(), ToolStart, map[string]any{"action": StartCheck, "name": "report"})
	if !handled {
		t.Fatalf("ExecuteTool(%q) handled = false, want true", ToolStart)
	}
	if res.IsError {
		t.Fatalf("ExecuteTool(%q) result = %#v, want success", ToolStart, res)
	}
	load, _ := rt.ExecuteTool(context.Background(), ToolLoad, map[string]any{"name": "report"})
	if load.IsError || load.Content == "" {
		t.Fatalf("ExecuteTool(%q) result = %#v, want enabled skill content", ToolLoad, load)
	}
	if !store.trust["report"].Enabled {
		t.Fatalf("store.trust[report].Enabled = false, want true")
	}
	if got := len(prompter.questions); got != 2 {
		t.Fatalf("len(prompter.questions) = %d, want %d", got, 2)
	}
}

func TestRuntimeImportLocalSkillRequiresExplicitEnable(t *testing.T) {
	root := t.TempDir()
	source := writeSourceSkill(t, "imported", "imported", "Imported.")
	rt := NewRuntime(root, &memoryStore{})

	res, err := rt.Import(context.Background(), source, "")
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !res.Check.Valid {
		t.Fatalf("Import() check = %#v, want valid", res.Check)
	}
	assertFileExists(t, filepath.Join(root, "imported", "SKILL.md"))
	load, _ := rt.ExecuteTool(context.Background(), ToolLoad, map[string]any{"name": "imported"})
	if !load.IsError {
		t.Fatalf("ExecuteTool(%q) result = %#v, want error before explicit enable", ToolLoad, load)
	}
}

func TestRuntimeStartImportRunsWorkflow(t *testing.T) {
	root := t.TempDir()
	source := writeSourceSkill(t, "imported", "imported", "Imported.")
	store := &memoryStore{}
	prompter := &fakePrompter{answers: []string{optionReviewNo, optionEnableYes}}
	rt := newRuntimeWithPrompter(root, store, prompter)

	res, handled := rt.ExecuteTool(context.Background(), ToolStart, map[string]any{"action": StartImport, "source": source})
	if !handled {
		t.Fatalf("ExecuteTool(%q) handled = false, want true", ToolStart)
	}
	if res.IsError {
		t.Fatalf("ExecuteTool(%q) result = %#v, want success", ToolStart, res)
	}
	if !store.trust["imported"].Enabled {
		t.Fatalf("store.trust[imported].Enabled = false, want true")
	}
	if got := len(prompter.questions); got != 2 {
		t.Fatalf("len(prompter.questions) = %d, want %d", got, 2)
	}
}

func TestRuntimeStartChoiceRetry(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "retry", "retry", "Retry choices.")
	prompter := &fakePrompter{answers: []string{"anything", optionReviewNo, "not sure", optionEnableNo}}
	rt := newRuntimeWithPrompter(root, &memoryStore{trust: map[string]Record{"retry": {Enabled: false}}}, prompter)

	res, handled := rt.ExecuteTool(context.Background(), ToolStart, map[string]any{"action": StartCheck, "name": "retry"})
	if !handled {
		t.Fatalf("ExecuteTool(%q) handled = false, want true", ToolStart)
	}
	if res.IsError {
		t.Fatalf("ExecuteTool(%q) result = %#v, want success after choice retry", ToolStart, res)
	}
	if got := len(prompter.questions); got != 4 {
		t.Fatalf("len(prompter.questions) = %d, want %d", got, 4)
	}
}

func TestRuntimeSetEnabledDoesNotRunCheck(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "toggle", "toggle-skill", "Toggle skill.")
	writeFile(t, filepath.Join(root, "toggle", "scripts", "run.sh"), "curl https://example.com\n")
	store := &memoryStore{trust: map[string]Record{"toggle-skill": {Enabled: false, Reasons: []string{"old reason"}}}}
	rt := NewRuntime(root, store)

	if err := rt.SetEnabled(context.Background(), EnableDecision{Name: "toggle-skill", Enabled: true}); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	if !store.trust["toggle-skill"].Enabled {
		t.Fatalf("store.trust[toggle-skill].Enabled = false, want true")
	}
	if got := store.trust["toggle-skill"].Reasons; len(got) != 1 || got[0] != "old reason" {
		t.Fatalf("store.trust[toggle-skill].Reasons = %#v, want [old reason]", got)
	}
}

func TestRuntimeStartEnableUsesExistingWorkflowCheck(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "flow", "flow-skill", "Flow skill.")
	store := &memoryStore{trust: map[string]Record{"flow-skill": {Enabled: false}}}
	prompter := &fakePrompter{answers: []string{optionReviewNo, optionEnableYes}}
	rt := newRuntimeWithPrompter(root, store, prompter)

	res, handled := rt.ExecuteTool(context.Background(), ToolStart, map[string]any{"action": StartCheck, "name": "flow-skill"})
	if !handled {
		t.Fatalf("ExecuteTool(%q) handled = false, want true", ToolStart)
	}
	if res.IsError {
		t.Fatalf("ExecuteTool(%q) result = %#v, want success", ToolStart, res)
	}
	writeFile(t, filepath.Join(root, "flow", "scripts", "late.sh"), "curl https://example.com\n")
	if got := store.trust["flow-skill"].Reasons; len(got) != 0 {
		t.Fatalf("store.trust[flow-skill].Reasons = %#v, want empty original check reasons", got)
	}
}

func TestManagerDuplicateSkillNameInvalid(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "one", "same-skill", "First.")
	writeSkill(t, root, "two", "same-skill", "Second.")
	m := NewManager(root, map[string]Record{"same-skill": {Enabled: true}})

	if err := m.Reload(context.Background()); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	infos := m.List()
	if got := len(infos); got != 1 {
		t.Fatalf("len(infos) = %d, want %d", got, 1)
	}
	if infos[0].Valid {
		t.Fatalf("infos[0].Valid = true, want false for duplicate skill")
	}
	if _, ok, reason := m.Load("same-skill"); ok || reason == "" {
		t.Fatalf("Load() ok = %v, reason = %q, want blocked duplicate", ok, reason)
	}
}

func TestRuntimeDisableMissingRecordedSkill(t *testing.T) {
	store := &memoryStore{trust: map[string]Record{"gone": {Enabled: true, Reasons: []string{"old"}}}}
	rt := NewRuntime(t.TempDir(), store)

	if err := rt.Disable(context.Background(), "gone"); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if store.trust["gone"].Enabled {
		t.Fatalf("store.trust[gone].Enabled = true, want false")
	}
}

func TestRuntimeImportRejectsInstalledSource(t *testing.T) {
	root := t.TempDir()
	installed := writeInstalledSkill(t, root, "same", "same", "Same.")
	rt := NewRuntime(root, &memoryStore{})

	if _, err := rt.Import(context.Background(), installed, "same"); err == nil {
		t.Fatalf("Import() error = nil, want non-nil for installed source")
	}
	assertFileExists(t, filepath.Join(installed, "SKILL.md"))
}

func TestLoadNotificationFromResult(t *testing.T) {
	res := tool.TextResult("loaded")
	res.Metadata = map[string]any{"skill_name": "writer"}

	evt, ok := LoadNotificationFromResult(ToolLoad, map[string]any{}, res)
	if !ok {
		t.Fatalf("LoadNotificationFromResult() ok = false, want true")
	}
	if evt.Name != "writer" {
		t.Fatalf("event.Name = %q, want %q", evt.Name, "writer")
	}
}

func TestRuntimeOptionalLLMReview(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "review", "review", "Review me.")
	reviewer := &fakeReviewer{response: "看起来可用，未发现明显风险。"}
	rt := NewRuntime(root, &memoryStore{})
	rt.SetReviewer(reviewer)

	res, err := rt.Review(context.Background(), "review")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if !res.Valid || res.Review == "" {
		t.Fatalf("Review() result = %#v, want valid review", res)
	}
	if reviewer.seen.Name != "review" {
		t.Fatalf("reviewer.seen.Name = %q, want %q", reviewer.seen.Name, "review")
	}
}

func TestRuntimeReviewRequiresReviewer(t *testing.T) {
	rt := NewRuntime(t.TempDir(), &memoryStore{})
	if _, err := rt.Review(context.Background(), "missing"); err == nil {
		t.Fatalf("Review() error = nil, want non-nil without reviewer")
	}
}

func newRuntimeWithPrompter(root string, store *memoryStore, prompter *fakePrompter) *Runtime {
	rt := NewRuntime(root, store)
	rt.SetPrompter(prompter)
	return rt
}

func writeSourceSkill(t *testing.T, dir, name, desc string) string {
	t.Helper()
	root := t.TempDir()
	writeSkill(t, root, dir, name, desc)
	return filepath.Join(root, dir)
}

func writeInstalledSkill(t *testing.T, root, dir, name, desc string) string {
	t.Helper()
	writeSkill(t, root, dir, name, desc)
	return filepath.Join(root, dir)
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
}

type memoryStore struct{ trust map[string]Record }

type fakePrompter struct {
	answers   []string
	questions []string
}

func (f *fakePrompter) AskChoice(ctx context.Context, question string, options []string) (string, error) {
	_ = ctx
	f.questions = append(f.questions, question)
	if len(f.answers) == 0 {
		return "", nil
	}
	answer := f.answers[0]
	f.answers = f.answers[1:]
	return answer, nil
}

type fakeReviewer struct {
	response string
	seen     LLMReviewRequest
}

func (f *fakeReviewer) ReviewSkill(ctx context.Context, req LLMReviewRequest) (string, error) {
	_ = ctx
	f.seen = req
	return f.response, nil
}

func (s *memoryStore) LoadSkillRecords() map[string]Record {
	out := make(map[string]Record, len(s.trust))
	for k, v := range s.trust {
		out[k] = v
	}
	return out
}

func (s *memoryStore) SaveSkillRecords(trust map[string]Record) error {
	s.trust = make(map[string]Record, len(trust))
	for k, v := range trust {
		s.trust[k] = v
	}
	return nil
}
