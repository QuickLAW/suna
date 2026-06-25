package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alanchenchen/suna/internal/config"
	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
	"github.com/alanchenchen/suna/internal/protocol"
)

type ConfigSetParams struct {
	Action       string
	Model        ConfigModel
	ModelRef     string
	ActiveModel  string
	APIKey       string
	DeleteAPIKey bool
	Locale       string
	Theme        string
	GuardMode    string
	Workspace    *string
	// MaxModelRPS 高级设置：nil 表示不更新；非 nil 时直接覆盖。
	MaxModelRPS *int
	// Scope 决定本次写入哪个配置文件："global" / "project" / ""。
	// 空字符串时按字段默认作用域推断（model 相关写全局，update_general 字段按前端意图）。
	Scope string
}

type ConfigModel struct {
	Provider        string
	Model           string
	BaseURL         string
	ContextWindow   int
	MaxOutputTokens int
	Strengths       []string
	SubtaskFor      []string
	Reasoning       map[string]any
}

func (a *Agent) Config() *config.Config {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.cfg.Clone()
}

func (a *Agent) ReloadConfigFromDiskIfNeeded() (*config.Config, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	// 优先用项目级路径（如果有）作为 mtime 判定的依据；
	// 项目级不存在时只关注全局 mtime。
	paths := []string{a.cfg.GlobalConfigPath}
	if a.cfg.ProjectConfigPath != "" {
		paths = append(paths, a.cfg.ProjectConfigPath)
	}
	var newestModTime time.Time
	anyExists := false
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		anyExists = true
		if info.ModTime().After(newestModTime) {
			newestModTime = info.ModTime()
		}
	}
	if !anyExists {
		// 首次启动时全局 + 项目级 config.toml 都还不存在：保持内存中的空配置。
		return a.cfg.Clone(), nil
	}
	if !newestModTime.After(a.configModTime) && len(a.cfg.Models) > 0 {
		return a.cfg.Clone(), nil
	}
	loaded, err := config.LoadWithProject(a.cfg.GlobalConfigPath, a.cfg.ProjectConfigPath)
	if err != nil {
		return a.cfg.Clone(), err
	}
	a.cfg = loaded
	a.configModTime = newestModTime
	if err := a.reloadRouterLocked(loaded); err != nil {
		return nil, err
	}
	a.guard = a.newGuardForSession(a.sessionID)
	a.reloadSkillsLocked()
	if a.mcp != nil {
		a.mcp.SetConfig(loaded.MCP)
	}
	if a.tools != nil {
		if err := a.tools.Reload(context.Background()); err != nil {
			return a.cfg.Clone(), err
		}
	}
	return a.cfg.Clone(), nil
}

func (a *Agent) UpdateConfig(params ConfigSetParams) (*config.Config, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg := a.cfg.Clone()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	switch params.Action {
	case protocol.ConfigActionUpsertModel:
		mc := config.ModelConfig{Provider: params.Model.Provider, Model: params.Model.Model, BaseURL: params.Model.BaseURL, ContextWindow: params.Model.ContextWindow, MaxOutputTokens: params.Model.MaxOutputTokens, Strengths: append([]string(nil), params.Model.Strengths...), SubtaskFor: append([]string(nil), params.Model.SubtaskFor...), Reasoning: cloneMap(params.Model.Reasoning)}
		if mc.Provider == "" || mc.Model == "" {
			return nil, fmt.Errorf("provider and model are required")
		}
		ref := mc.Ref()
		updated := false
		for i, existing := range cfg.Models {
			if existing.Ref() == params.ModelRef || existing.Ref() == ref {
				mc.APIKey = existing.APIKey
				cfg.Models[i] = mc
				updated = true
				break
			}
		}
		if !updated {
			cfg.Models = append(cfg.Models, mc)
		}
		if cfg.ActiveModel == "" || cfg.ActiveModel == params.ModelRef {
			cfg.ActiveModel = ref
		}
		if params.ActiveModel != "" {
			cfg.ActiveModel = params.ActiveModel
		}
		if params.APIKey != "" {
			if err := config.SaveCredential(cfg.DataDir, mc.Provider, params.APIKey); err != nil {
				return nil, err
			}
		}
	case protocol.ConfigActionDeleteModel:
		if params.ModelRef == "" {
			return nil, fmt.Errorf("model_ref is required")
		}
		deletedProvider := ""
		filtered := cfg.Models[:0]
		for _, mc := range cfg.Models {
			if mc.Ref() != params.ModelRef {
				filtered = append(filtered, mc)
			} else {
				deletedProvider = mc.Provider
			}
		}
		cfg.Models = filtered
		if params.DeleteAPIKey && deletedProvider != "" && !providerStillUsed(cfg.Models, deletedProvider) {
			if err := config.DeleteCredential(cfg.DataDir, deletedProvider); err != nil {
				return nil, err
			}
		}
		if cfg.ActiveModel == params.ModelRef {
			// 删的是当前 active 时，前端可以传 params.ActiveModel 指定新 active。
			// 校验：必须是删除后剩下的某个 model ref；非法或为空都 fallback 到 Models[0]，
			// 与"用户没传 ActiveModel" 的旧行为一致。
			newActive := params.ActiveModel
			valid := false
			if newActive != "" {
				for _, mc := range cfg.Models {
					if mc.Ref() == newActive {
						valid = true
						break
					}
				}
			}
			if valid {
				cfg.ActiveModel = newActive
			} else {
				cfg.ActiveModel = ""
				if len(cfg.Models) > 0 {
					cfg.ActiveModel = cfg.Models[0].Ref()
				}
			}
		}
	case protocol.ConfigActionActivateModel:
		if _, ok := cfg.ModelByRef(params.ActiveModel); !ok {
			return nil, fmt.Errorf("model %q not found", params.ActiveModel)
		}
		cfg.ActiveModel = params.ActiveModel
	case protocol.ConfigActionUpdateGeneral:
		if params.Locale != "" {
			cfg.UI.Locale = params.Locale
		}
		if params.Theme != "" {
			cfg.UI.Theme = params.Theme
		}
		if params.GuardMode != "" {
			cfg.Guard.Mode = config.GuardConfig{Mode: params.GuardMode}.ModeOrDefault()
		}
		if params.Workspace != nil {
			cfg.Guard.Workspace = *params.Workspace
		}
		if params.MaxModelRPS != nil {
			n := *params.MaxModelRPS
			if n < 0 {
				return nil, fmt.Errorf("max_model_rps must be non-negative")
			}
			cfg.MaxModelRPS = n
		}
	default:
		return nil, fmt.Errorf("unknown config action %q", params.Action)
	}
	if err := a.commitConfigUpdate(cfg, params); err != nil {
		return nil, err
	}
	if err := config.LoadCredentials(cfg); err != nil {
		return nil, err
	}
	a.cfg = cfg
	if err := a.reloadRouterLocked(cfg); err != nil {
		return nil, err
	}
	a.guard = a.newGuardForSession(a.sessionID)
	a.reloadSkillsLocked()
	if a.tools != nil {
		if err := a.tools.Reload(context.Background()); err != nil {
			return nil, err
		}
	}
	a.refreshConfigModTimeLocked()
	return cfg, nil
}

// refreshConfigModTimeLocked 重新记录全局 + 项目级 cfg 的最新 mtime。
// UpdateConfig 写完文件后调用，确保下一次 ReloadConfigFromDiskIfNeeded 不会
// 因为 mtime 漂移而误判为有变更。
func (a *Agent) refreshConfigModTimeLocked() {
	now := time.Time{}
	for _, p := range []string{a.cfg.GlobalConfigPath, a.cfg.ProjectConfigPath} {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.ModTime().After(now) {
			now = info.ModTime()
		}
	}
	if !now.IsZero() {
		a.configModTime = now
	}
}

// commitConfigUpdate 把 UpdateConfig 改完字段后的 cfg 持久化到对应文件。
// params.Scope 决定目标文件；空值时默认写全局。
// 项目级写入时，只把本次改动涉及的字段写入 ProjectOverrides，避免全量复制 merged 视图
// 导致所有字段都被标记为"项目级覆盖"。
func (a *Agent) commitConfigUpdate(cfg *config.Config, params ConfigSetParams) error {
	scope := params.Scope
	if scope == "" {
		scope = "global"
	}
	if scope == "project" {
		return a.commitProjectUpdate(cfg, params)
	}
	return a.commitGlobalUpdate(cfg, params)
}

// commitProjectUpdate 把本次 UpdateConfig 改动的字段写入 ProjectOverrides 并保存。
func (a *Agent) commitProjectUpdate(cfg *config.Config, params ConfigSetParams) error {
	if cfg.ProjectConfigPath == "" {
		return fmt.Errorf("config: no project config path; cannot write project scope")
	}
	if cfg.ProjectOverrides == nil {
		cfg.ProjectOverrides = &config.Config{}
	}
	p := cfg.ProjectOverrides
	p.GlobalConfigPath = cfg.GlobalConfigPath
	p.ProjectConfigPath = cfg.ProjectConfigPath
	// 按 action 只更新本次改动的字段，不触碰 ProjectOverrides 的其余内容。
	switch params.Action {
	case protocol.ConfigActionUpsertModel, protocol.ConfigActionDeleteModel, protocol.ConfigActionActivateModel:
		p.Models = append([]config.ModelConfig(nil), cfg.Models...)
		p.ActiveModel = cfg.ActiveModel
	case protocol.ConfigActionUpdateGeneral:
		if params.Locale != "" {
			p.UI.Locale = cfg.UI.Locale
		}
		if params.Theme != "" {
			p.UI.Theme = cfg.UI.Theme
		}
		if params.GuardMode != "" {
			p.Guard.Mode = cfg.Guard.Mode
		}
		if params.Workspace != nil {
			p.Guard.Workspace = cfg.Guard.Workspace
		}
		if params.MaxModelRPS != nil {
			p.MaxModelRPS = cfg.MaxModelRPS
		}
	}
	return cfg.SaveToProject()
}

// commitGlobalUpdate 更新 GlobalView 中本次改动的字段，然后保存到全局文件。
func (a *Agent) commitGlobalUpdate(cfg *config.Config, params ConfigSetParams) error {
	if cfg.GlobalView == nil {
		cfg.GlobalView = &config.Config{}
	}
	g := cfg.GlobalView
	g.GlobalConfigPath = cfg.GlobalConfigPath
	g.ProjectConfigPath = cfg.ProjectConfigPath
	// 按 action 更新 GlobalView 中被改动的字段。
	switch params.Action {
	case protocol.ConfigActionUpsertModel, protocol.ConfigActionDeleteModel, protocol.ConfigActionActivateModel:
		g.Models = append([]config.ModelConfig(nil), cfg.Models...)
		g.ActiveModel = cfg.ActiveModel
	case protocol.ConfigActionUpdateGeneral:
		if params.Locale != "" {
			g.UI.Locale = cfg.UI.Locale
		}
		if params.Theme != "" {
			g.UI.Theme = cfg.UI.Theme
		}
		if params.GuardMode != "" {
			g.Guard.Mode = cfg.Guard.Mode
		}
		if params.Workspace != nil {
			g.Guard.Workspace = cfg.Guard.Workspace
		}
		if params.MaxModelRPS != nil {
			g.MaxModelRPS = cfg.MaxModelRPS
		}
	}
	return cfg.SaveToGlobal()
}

func (a *Agent) reloadSkillsLocked() {
	if a.cfg == nil || a.skills == nil {
		return
	}
	a.skills.SetRoot(a.cfg.SkillsDir())
	a.skills.SetStore(a.cfg)
	a.skills.SetReviewer(agentSkillReviewer{})
	a.skills.SetPrompter(agentSkillPrompter{})
	_ = a.skills.Reload(context.Background())
}

func providerStillUsed(models []config.ModelConfig, provider string) bool {
	for _, mc := range models {
		if mc.Provider == provider {
			return true
		}
	}
	return false
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (a *Agent) reloadRouterLocked(cfg *config.Config) error {
	if len(cfg.Models) == 0 || cfg.ActiveModel == "" {
		a.router = nil
		a.compressor = memory.NewCompressor(nil)
		if a.prompts != nil {
			a.compressor.SetPrompts(a.prompts)
		}
		if a.extractWorker != nil {
			a.extractWorker.SetProvider(nil)
		}
		return nil
	}
	router, err := model.NewRouter(cfg, a.mediaStore)
	if err != nil {
		return err
	}
	a.router = router
	if a.prompts != nil {
		router.SetPrompts(a.prompts)
	}
	provider := model.NewRoutedProvider(router)
	a.compressor = memory.NewCompressor(provider)
	if a.prompts != nil {
		a.compressor.SetPrompts(a.prompts)
	}
	if a.extractWorker != nil {
		a.extractWorker.SetProvider(provider)
	}
	return nil
}
