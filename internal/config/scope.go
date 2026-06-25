package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/alanchenchen/suna/internal/skill"
)

// ConfigSources 标记每个业务字段当前生效值来自哪个配置文件。
// 值是 "global" 或 "project"，用于前端展示作用域徽标。
//
// 字段之间粒度尽量小：Guard.Mode 与 Guard.Workspace 各自一个标记，
// 这样 UI 可对单字段做"切回全局"或"切到项目级"的操作。
type ConfigSources struct {
	ActiveModel    string `json:"active_model"`
	Models         string `json:"models"`
	GuardMode      string `json:"guard_mode"`
	GuardWorkspace string `json:"guard_workspace"`
	UITheme        string `json:"ui_theme"`
	UILocale       string `json:"ui_locale"`
	Skills         string `json:"skills"`
	MCP            string `json:"mcp"`
	Hooks          string `json:"hooks"`
	MaxModelRPS    string `json:"max_model_rps"`
}

// AllGlobalSources 构造一个全部字段都标记为 "global" 的 Sources。
func AllGlobalSources() ConfigSources {
	return ConfigSources{
		ActiveModel:    "global",
		Models:         "global",
		GuardMode:      "global",
		GuardWorkspace: "global",
		UITheme:        "global",
		UILocale:       "global",
		Skills:         "global",
		MCP:            "global",
		Hooks:          "global",
		MaxModelRPS:    "global",
	}
}

// loadRawConfig 从 path 解码一个 Config，不做业务校验，也不要求路径存在。
// 不调 NormalizeUI：项目级文件里"未设"的字段必须保持零值字符串，否则 applyProjectOverrides
// 会被 "auto"/"en" 这类默认值误判为"项目级显式覆盖"。
// 返回的 cfg 仅含"项目级文件原始内容"，后续由 applyProjectOverrides 合入全局。
func loadRawConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// LoadWithProject 加载全局 config.toml，再叠加项目级 .suna/config.toml（字段级合并）。
// projectPath 为空时退化为 Load(globalPath)，等同于无项目级覆盖。
// 项目级文件存在但解析失败会返回错误（不同于项目级文件不存在：后者视为无覆盖）。
func LoadWithProject(globalPath, projectPath string) (*Config, error) {
	global, err := Load(globalPath)
	if err != nil {
		return nil, err
	}
	global.GlobalConfigPath = globalPath
	global.Sources = AllGlobalSources()
	// GlobalView 必须在 applyProjectOverrides 之前克隆，否则它会变成 merged 视图
	// 而不是"全局真实值"，后续 SaveToGlobal 就退化为"写 merged"。
	global.GlobalView = global.Clone()
	if projectPath == "" {
		return global, nil
	}
	project, perr := loadRawConfig(projectPath)
	if perr != nil {
		if errors.Is(perr, os.ErrNotExist) {
			return global, nil
		}
		return nil, fmt.Errorf("load project config %s: %w", projectPath, perr)
	}
	global.ProjectConfigPath = projectPath
	global.ProjectOverrides = project
	applyProjectOverrides(global, project)
	return global, nil
}

// applyProjectOverrides 把 project 字段级合并到 global（直接修改 global 本身）。
// 规则：项目级“非零”字段覆盖全局对应字段；项目级未设置的字段保持全局值。
// 合并完成后通过 Sources 标记每个被覆盖的字段为 "project"。
func applyProjectOverrides(global, project *Config) {
	if project == nil || global == nil {
		return
	}
	src := global.Sources
	if project.ActiveModel != "" {
		global.ActiveModel = project.ActiveModel
		src.ActiveModel = "project"
	}
	if len(project.Models) > 0 {
		global.Models = append([]ModelConfig(nil), project.Models...)
		src.Models = "project"
	}
	if project.Guard.Mode != "" {
		global.Guard.Mode = project.Guard.Mode
		src.GuardMode = "project"
	}
	if project.Guard.Workspace != "" {
		global.Guard.Workspace = project.Guard.Workspace
		src.GuardWorkspace = "project"
	}
	if len(project.Guard.Blocked) > 0 {
		global.Guard.Blocked = append([]GuardRule(nil), project.Guard.Blocked...)
	}
	if len(project.Guard.Allowed) > 0 {
		global.Guard.Allowed = append([]GuardAllowRule(nil), project.Guard.Allowed...)
	}
	if project.UI.Theme != "" {
		global.UI.Theme = project.UI.Theme
		src.UITheme = "project"
	}
	if project.UI.Locale != "" {
		global.UI.Locale = project.UI.Locale
		src.UILocale = "project"
	}
	if len(project.Skills) > 0 {
		if global.Skills == nil {
			global.Skills = map[string]skill.Record{}
		}
		for name, ps := range project.Skills {
			existing, ok := global.Skills[name]
			if !ok {
				existing = skill.Record{}
			}
			existing.Enabled = ps.Enabled
			if len(ps.Reasons) > 0 {
				existing.Reasons = append([]string(nil), ps.Reasons...)
			}
			global.Skills[name] = existing
		}
		src.Skills = "project"
	}
	if len(project.MCP.Servers) > 0 {
		if global.MCP.Servers == nil {
			global.MCP.Servers = map[string]MCPServerConfig{}
		}
		for name, ps := range project.MCP.Servers {
			existing := global.MCP.Servers[name]
			existing.Enabled = ps.Enabled
			global.MCP.Servers[name] = existing
		}
		src.MCP = "project"
	}
	if len(project.Hooks) > 0 {
		global.Hooks = append([]HookConfig(nil), project.Hooks...)
		src.Hooks = "project"
	}
	if project.MaxModelRPS > 0 {
		global.MaxModelRPS = project.MaxModelRPS
		src.MaxModelRPS = "project"
	}
	global.Sources = src
}

// IsProjectScoped 报告当前是否有任何字段被项目级覆盖。
func (s ConfigSources) IsProjectScoped() bool {
	return s.ActiveModel == "project" || s.Models == "project" ||
		s.GuardMode == "project" || s.GuardWorkspace == "project" ||
		s.UITheme == "project" || s.UILocale == "project" ||
		s.Skills == "project" || s.MCP == "project" ||
		s.Hooks == "project" || s.MaxModelRPS == "project"
}
