package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alanchenchen/suna/internal/config"
	"github.com/alanchenchen/suna/internal/protocol"
)

// TestUpdateConfigFromEmptyDirWritesFiles 端到端：模拟 Suna 首次启动场景，
// 全局 config.toml 和 credentials.toml 都不存在时，调 agent.UpdateConfig
// upsert_model 应当成功创建 config.toml + credentials.toml。
// 这正是用户在 TUI 首次进入"添加模型"流程时实际走的路径。
func TestUpdateConfigFromEmptyDirWritesFiles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	credPath := filepath.Join(dir, "credentials.toml")
	// 确认两个文件都不存在
	if _, err := os.Stat(cfgPath); err == nil {
		t.Fatal("config.toml should not exist before save")
	}
	if _, err := os.Stat(credPath); err == nil {
		t.Fatal("credentials.toml should not exist before save")
	}

	cfg := &config.Config{
		GlobalConfigPath:  cfgPath,
		ProjectConfigPath: "",
		DataDir:           dir,
	}
	a := &Agent{cfg: cfg}

	updated, err := a.UpdateConfig(ConfigSetParams{
		Action:   protocol.ConfigActionUpsertModel,
		APIKey:   "sk-test-1234567890",
		Model: ConfigModel{
			Provider:        "openai",
			Model:           "gpt-4o-mini",
			BaseURL:         "https://api.openai.com/v1",
			ContextWindow:   128000,
			MaxOutputTokens: 8192,
		},
		ActiveModel: "openai/gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	if len(updated.Models) != 1 {
		t.Fatalf("updated.Models len = %d, want 1", len(updated.Models))
	}
	if updated.ActiveModel != "openai/gpt-4o-mini" {
		t.Errorf("active_model = %q, want openai/gpt-4o-mini", updated.ActiveModel)
	}

	// 必须真的写了 config.toml
	cfgBody, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.toml after save: %v", err)
	}
	t.Logf("config.toml after save:\n%s", string(cfgBody))
	if !strings.Contains(string(cfgBody), "gpt-4o-mini") {
		t.Errorf("config.toml should contain model name 'gpt-4o-mini', got:\n%s", string(cfgBody))
	}
	if !strings.Contains(string(cfgBody), "openai") {
		t.Errorf("config.toml should contain provider 'openai', got:\n%s", string(cfgBody))
	}
	if !strings.Contains(string(cfgBody), "https://api.openai.com/v1") {
		t.Errorf("config.toml should contain base_url, got:\n%s", string(cfgBody))
	}

	// 必须真的写了 credentials.toml
	credBody, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read credentials.toml after save: %v", err)
	}
	t.Logf("credentials.toml after save:\n%s", string(credBody))
	if !strings.Contains(string(credBody), "sk-test-1234567890") {
		t.Errorf("credentials.toml should contain api_key, got:\n%s", string(credBody))
	}
}

// TestUpdateConfigWithoutAPIKey 验证空 APIKey 也能保存（编辑模型时留空 = 不覆盖）。
func TestUpdateConfigWithoutAPIKey(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		GlobalConfigPath:  filepath.Join(dir, "config.toml"),
		ProjectConfigPath: "",
		DataDir:           dir,
	}
	// 先写一次带 key
	a := &Agent{cfg: cfg}
	if _, err := a.UpdateConfig(ConfigSetParams{
		Action: protocol.ConfigActionUpsertModel,
		APIKey: "sk-original",
		Model: ConfigModel{
			Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com/v1",
			ContextWindow: 128000, MaxOutputTokens: 8192,
		},
		ActiveModel: "openai/gpt-4o",
	}); err != nil {
		t.Fatalf("first UpdateConfig error = %v", err)
	}
	// 再编辑，不带 key
	if _, err := a.UpdateConfig(ConfigSetParams{
		Action:   protocol.ConfigActionUpsertModel,
		ModelRef: "openai/gpt-4o",
		APIKey:   "", // 留空 = 不覆盖
		Model: ConfigModel{
			Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com/v1",
			ContextWindow: 200000, MaxOutputTokens: 16384,
		},
	}); err != nil {
		t.Fatalf("second UpdateConfig error = %v", err)
	}
	// 验证原 key 保留
	body, err := os.ReadFile(filepath.Join(dir, "credentials.toml"))
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if !strings.Contains(string(body), "sk-original") {
		t.Errorf("credentials should still contain original key, got:\n%s", string(body))
	}
	// 验证新 context_window 写入
	cfgBody, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfgBody), "200000") {
		t.Errorf("config.toml should contain updated context_window 200000, got:\n%s", string(cfgBody))
	}
}

// TestUpdateConfigProjectScopeWritesProjectFile 验证 Scope="project" 时写到项目级文件。
func TestUpdateConfigProjectScopeWritesProjectFile(t *testing.T) {
	dir := t.TempDir()
	cwd := t.TempDir()
	cfg := &config.Config{
		GlobalConfigPath:  filepath.Join(dir, "config.toml"),
		ProjectConfigPath: filepath.Join(cwd, ".suna", "config.toml"),
		DataDir:           dir,
	}
	a := &Agent{cfg: cfg}

	if _, err := a.UpdateConfig(ConfigSetParams{
		Action: protocol.ConfigActionUpsertModel,
		APIKey: "sk-test",
		Model: ConfigModel{
			Provider: "openai", Model: "gpt-4o", BaseURL: "https://api.openai.com/v1",
			ContextWindow: 128000, MaxOutputTokens: 8192,
		},
		ActiveModel: "openai/gpt-4o",
		Scope:       "project",
	}); err != nil {
		t.Fatalf("UpdateConfig project scope error = %v", err)
	}

	// 全局文件不应写入
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err == nil {
		t.Error("global config.toml should NOT be created when Scope=project")
	}
	// 项目级文件应写入
	projPath := filepath.Join(cwd, ".suna", "config.toml")
	if _, err := os.Stat(projPath); err != nil {
		t.Errorf("project config.toml should be created at %s: %v", projPath, err)
	}
}
