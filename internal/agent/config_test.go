package agent

import (
	"testing"

	"github.com/alanchenchen/suna/internal/config"
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
