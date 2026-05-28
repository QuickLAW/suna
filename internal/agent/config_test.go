package agent

import (
	"context"
	"testing"

	"github.com/alanchenchen/suna/internal/config"
	"github.com/alanchenchen/suna/internal/media"
	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
	"github.com/alanchenchen/suna/internal/protocol"
)

func TestUpdateConfigDeleteModelKeepsCredentialByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		ActiveModel: "openai/gpt-4o-mini",
		Models:      []config.ModelConfig{{Provider: "openai", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKey: "sk-openai"}},
		UI:          config.UIConfig{Theme: "auto", Locale: "en"},
		Guard:       config.GuardConfig{Mode: "ask"},
		DataDir:     dir,
	}
	if err := config.SaveCredential(dir, "openai", "sk-openai"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	a := &Agent{cfg: cfg}

	if _, err := a.UpdateConfig(ConfigSetParams{Action: protocol.ConfigActionDeleteModel, ModelRef: "openai/gpt-4o-mini"}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	loaded := &config.Config{Models: []config.ModelConfig{{Provider: "openai", Model: "gpt-4o-mini"}}, DataDir: dir}
	if err := config.LoadCredentials(loaded); err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got := loaded.Models[0].APIKey; got != "sk-openai" {
		t.Fatalf("APIKey = %q", got)
	}
}

func TestUpdateConfigDeleteLastProviderModelCanDeleteCredential(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		ActiveModel: "openai/gpt-4o-mini",
		Models:      []config.ModelConfig{{Provider: "openai", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKey: "sk-openai"}},
		UI:          config.UIConfig{Theme: "auto", Locale: "en"},
		Guard:       config.GuardConfig{Mode: "ask"},
		DataDir:     dir,
	}
	if err := config.SaveCredential(dir, "openai", "sk-openai"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	a := &Agent{cfg: cfg}

	if _, err := a.UpdateConfig(ConfigSetParams{Action: protocol.ConfigActionDeleteModel, ModelRef: "openai/gpt-4o-mini", DeleteAPIKey: true}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	loaded := &config.Config{Models: []config.ModelConfig{{Provider: "openai", Model: "gpt-4o-mini"}}, DataDir: dir}
	if err := config.LoadCredentials(loaded); err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got := loaded.Models[0].APIKey; got != "" {
		t.Fatalf("APIKey = %q, want empty", got)
	}
}

func TestUpdateConfigDoesNotDeleteCredentialWhenProviderStillUsed(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		ActiveModel: "openai/gpt-4o-mini",
		Models: []config.ModelConfig{
			{Provider: "openai", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKey: "sk-openai"},
			{Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com/v1", APIKey: "sk-openai"},
		},
		UI:      config.UIConfig{Theme: "auto", Locale: "en"},
		Guard:   config.GuardConfig{Mode: "ask"},
		DataDir: dir,
	}
	if err := config.SaveCredential(dir, "openai", "sk-openai"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	a := &Agent{cfg: cfg}

	if _, err := a.UpdateConfig(ConfigSetParams{Action: protocol.ConfigActionDeleteModel, ModelRef: "openai/gpt-4o-mini", DeleteAPIKey: true}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	loaded := &config.Config{Models: []config.ModelConfig{{Provider: "openai", Model: "gpt-4o"}}, DataDir: dir}
	if err := config.LoadCredentials(loaded); err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got := loaded.Models[0].APIKey; got != "sk-openai" {
		t.Fatalf("APIKey = %q", got)
	}
}

func TestReloadRouterUpdatesMemoryWorkerProvider(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		ActiveModel: "openai/gpt-4o-mini",
		Models: []config.ModelConfig{
			{Provider: "openai", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKey: "sk-openai"},
			{Provider: "anthropic", Model: "claude-sonnet", BaseURL: "https://api.anthropic.com", APIKey: "sk-anthropic"},
		},
		UI:      config.UIConfig{Theme: "auto", Locale: "en"},
		Guard:   config.GuardConfig{Mode: "ask"},
		DataDir: dir,
	}
	if err := cfg.Save(cfg.ConfigPath()); err != nil {
		t.Fatalf("Save config: %v", err)
	}
	if err := config.SaveCredential(dir, "openai", "sk-openai"); err != nil {
		t.Fatalf("SaveCredential openai: %v", err)
	}
	if err := config.SaveCredential(dir, "anthropic", "sk-anthropic"); err != nil {
		t.Fatalf("SaveCredential anthropic: %v", err)
	}
	store, err := memory.NewStore(memoryDBPath(t, dir))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	router, err := model.NewRouter(cfg, media.NewStore(t.TempDir()))
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	worker := memory.NewWorker(memory.NewExtractQueue(store.DB()), memory.NewMemoryStore(store.DB()), store.DB(), backgroundProvider(router))
	a := &Agent{cfg: cfg, router: router, mediaStore: media.NewStore(t.TempDir()), compressor: memory.NewCompressor(backgroundProvider(router)), extractWorker: worker}

	initial := worker.Provider()
	if initial == nil {
		t.Fatalf("initial provider is nil")
	}
	if _, ok := initial.(*model.OpenAIResponsesProvider); !ok {
		t.Fatalf("initial provider = %T, want OpenAIResponsesProvider", initial)
	}
	if _, err := a.UpdateConfig(ConfigSetParams{Action: protocol.ConfigActionActivateModel, ActiveModel: "anthropic/claude-sonnet"}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	updated := worker.Provider()
	if updated == nil {
		t.Fatalf("updated provider is nil")
	}
	if _, ok := updated.(*model.AnthropicProvider); !ok {
		t.Fatalf("updated provider = %T, want AnthropicProvider", updated)
	}
	if updated == initial {
		t.Fatalf("worker provider was not replaced after active model switch")
	}
}

func TestReloadRouterClearsMemoryWorkerProviderWithoutActiveModel(t *testing.T) {
	provider := fakeProvider{}
	worker := memory.NewWorker(nil, nil, nil, provider)
	a := &Agent{extractWorker: worker}

	if err := a.reloadRouterLocked(&config.Config{}); err != nil {
		t.Fatalf("reloadRouterLocked: %v", err)
	}
	if got := worker.Provider(); got != nil {
		t.Fatalf("worker provider = %T, want nil", got)
	}
}

func memoryDBPath(t *testing.T, dir string) string {
	t.Helper()
	return config.DataDirDBPath(dir)
}

type fakeProvider struct{}

func (fakeProvider) Complete(context.Context, *model.CompletionRequest) (<-chan model.Chunk, error) {
	ch := make(chan model.Chunk)
	close(ch)
	return ch, nil
}

func (fakeProvider) EstimateTokens(string) int { return 0 }

func (fakeProvider) ContextWindow() int { return 128000 }
