package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindProjectConfigPath 验证从 cwd 向上查找最近的 .suna/config.toml。
// 行为：只在"最近一级"找到即返回；不存在任何一级时返回空字符串。
func TestFindProjectConfigPath(t *testing.T) {
	tmp := t.TempDir()
	// 准备目录结构：
	//   <tmp>/proj/.suna/config.toml   <- 应被命中
	//   <tmp>/proj/sub/                <- 启动 cwd
	proj := filepath.Join(tmp, "proj")
	sub := filepath.Join(proj, "sub")
	deeper := filepath.Join(sub, "deeper")
	for _, d := range []string{filepath.Join(proj, ".suna"), sub, deeper} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(proj, ".suna", "config.toml"), []byte("active_model = \"x\"\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"cwd_inside_project", deeper, filepath.Join(proj, ".suna", "config.toml")},
		{"cwd_at_project", proj, filepath.Join(proj, ".suna", "config.toml")},
		{"cwd_outside_no_match", tmp, ""},
		{"empty_cwd", "", ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := FindProjectConfigPath(tt.cwd)
			if got != tt.want {
				t.Fatalf("FindProjectConfigPath(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}

// TestLoadWithProjectMergesFields 验证全局 + 项目级字段级合并：项目级非零字段
// 覆盖全局；零值字段保持全局。Sources 标记覆盖来源。
func TestLoadWithProjectMergesFields(t *testing.T) {
	tmp := t.TempDir()
	// 真实存在的 workspace 目录，规避 Load 的路径可达性校验。
	wsDir := filepath.Join(tmp, "global-ws")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	globalPath := filepath.Join(tmp, "global.toml")
	projectPath := filepath.Join(tmp, "project.toml")

	globalTOML := `
active_model = "anthropic/claude-sonnet"
max_model_rps = 5

[[models]]
provider = "anthropic"
model = "claude-sonnet"
base_url = "https://api.anthropic.com"
context_window = 200000
max_output_tokens = 8192

[guard]
mode = "ask"
workspace = "` + wsDir + `"

[ui]
theme = "auto"
locale = "en"
`
	if err := os.WriteFile(globalPath, []byte(globalTOML), 0644); err != nil {
		t.Fatalf("write global: %v", err)
	}

	projectTOML := `
active_model = "openai/gpt-4o"

[guard]
mode = "auto"
`
	if err := os.WriteFile(projectPath, []byte(projectTOML), 0644); err != nil {
		t.Fatalf("write project: %v", err)
	}

	cfg, err := LoadWithProject(globalPath, projectPath)
	if err != nil {
		t.Fatalf("LoadWithProject: %v", err)
	}

	if cfg.ActiveModel != "openai/gpt-4o" {
		t.Fatalf("ActiveModel = %q, want %q (project override)", cfg.ActiveModel, "openai/gpt-4o")
	}
	if cfg.Guard.Mode != "auto" {
		t.Fatalf("Guard.Mode = %q, want %q (project override)", cfg.Guard.Mode, "auto")
	}
	if cfg.Guard.Workspace != wsDir {
		t.Fatalf("Guard.Workspace = %q, want %q (kept from global)", cfg.Guard.Workspace, wsDir)
	}
	if cfg.UI.Theme != "auto" {
		t.Fatalf("UI.Theme = %q, want %q (kept from global)", cfg.UI.Theme, "auto")
	}
	if cfg.MaxModelRPS != 5 {
		t.Fatalf("MaxModelRPS = %d, want 5 (kept from global)", cfg.MaxModelRPS)
	}
	if len(cfg.Models) != 1 {
		t.Fatalf("len(Models) = %d, want 1 (kept from global)", len(cfg.Models))
	}

	if cfg.Sources.ActiveModel != "project" {
		t.Errorf("Sources.ActiveModel = %q, want project", cfg.Sources.ActiveModel)
	}
	if cfg.Sources.GuardMode != "project" {
		t.Errorf("Sources.GuardMode = %q, want project", cfg.Sources.GuardMode)
	}
	if cfg.Sources.GuardWorkspace != "global" {
		t.Errorf("Sources.GuardWorkspace = %q, want global", cfg.Sources.GuardWorkspace)
	}
	if cfg.Sources.UITheme != "global" {
		t.Errorf("Sources.UITheme = %q, want global", cfg.Sources.UITheme)
	}
}

// TestLoadWithProjectMissingFile 验证项目级文件不存在时退化为纯全局加载。
func TestLoadWithProjectMissingFile(t *testing.T) {
	tmp := t.TempDir()
	globalPath := filepath.Join(tmp, "global.toml")
	missingProject := filepath.Join(tmp, "nope", "config.toml")
	if err := os.WriteFile(globalPath, []byte(`
active_model = "openai/m"

[[models]]
provider = "openai"
model = "m"
base_url = "u"
context_window = 100
max_output_tokens = 1
`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadWithProject(globalPath, missingProject)
	if err != nil {
		t.Fatalf("LoadWithProject: %v", err)
	}
	if cfg.ActiveModel != "openai/m" {
		t.Fatalf("ActiveModel = %q, want openai/m", cfg.ActiveModel)
	}
	if cfg.ProjectConfigPath != "" {
		t.Fatalf("ProjectConfigPath = %q, want empty", cfg.ProjectConfigPath)
	}
	if cfg.ProjectOverrides != nil {
		t.Fatalf("ProjectOverrides = %v, want nil", cfg.ProjectOverrides)
	}
}

// TestSaveToGlobalDoesNotLeakProjectFields 验证 SaveToGlobal 写出的全局 cfg 不含
// 项目级覆盖字段，避免全局文件被项目级字段污染。
func TestSaveToGlobalDoesNotLeakProjectFields(t *testing.T) {
	tmp := t.TempDir()
	globalPath := filepath.Join(tmp, "global.toml")
	projectPath := filepath.Join(tmp, "project.toml")

	if err := os.WriteFile(globalPath, []byte(`
active_model = "openai/global-default"
max_model_rps = 3

[[models]]
provider = "openai"
model = "global-default"
base_url = "u"
context_window = 100
max_output_tokens = 1

[ui]
locale = "en"
theme = "auto"
`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(projectPath, []byte(`
active_model = "openai/project-only"

[guard]
mode = "auto"
`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadWithProject(globalPath, projectPath)
	if err != nil {
		t.Fatalf("LoadWithProject: %v", err)
	}
	// merged 后 cfg.ActiveModel = "openai/project-only"，但全局文件仍应是 "openai/global-default"。
	if err := cfg.SaveToGlobal(); err != nil {
		t.Fatalf("SaveToGlobal: %v", err)
	}

	reloaded, err := Load(globalPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.ActiveModel != "openai/global-default" {
		t.Fatalf("reloaded global ActiveModel = %q, want %q (project field must not leak)",
			reloaded.ActiveModel, "openai/global-default")
	}
}
