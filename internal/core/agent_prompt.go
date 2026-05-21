package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/alanchenchen/suna/internal/config"
	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
	"github.com/alanchenchen/suna/internal/prompt"
)

/*
buildSystemPrompt 通过模板渲染系统提示词。

这里集中拼装稳定系统提示词：运行环境、项目配置、能力摘要。
active memory 是动态内容，会在 LLM 请求 messages 前部作为 internal context 注入，避免污染 system prompt cache。
Run 循环每轮调用前都会重新构建，确保异步记忆提取和能力加载能自然进入下一次模型请求。
*/
func (a *Agent) buildSystemPrompt(ctx context.Context) (string, error) {
	if a.systemPromptOverride != "" {
		return a.systemPromptOverride, nil
	}

	env := getEnvInfo()

	projectConfig := ""
	wd, _ := os.Getwd()
	for _, name := range []string{"SUNA.md", ".suna/AGENTS.md"} {
		if data, err := os.ReadFile(filepath.Join(wd, name)); err == nil {
			projectConfig = string(data)
			break
		}
	}

	capabilities := ""
	if a.caps != nil {
		capabilities = a.caps.Summary()
	}

	return a.prompts.RenderSystem(prompt.SystemPromptData{
		OS:            env["OS"],
		Arch:          env["Arch"],
		WorkDir:       env["WorkDir"],
		ActiveModel:   a.activeModelSummary(),
		ModelRouting:  a.modelRoutingSummary(),
		ProjectConfig: projectConfig,
		Capabilities:  capabilities,
	})
}

func (a *Agent) buildRequestMessages(ctx context.Context) []model.Message {
	msgs := a.working.Messages()
	if a.memories == nil {
		return msgs
	}
	// Active memory 是每轮可能变化的动态内容，不能拼进 system prompt，否则会破坏
	// prompt cache 的稳定前缀。这里把它作为一次性的 internal context 放到 messages 最前面，
	// 不写入 working memory，也不展示给 TUI。
	brief, _, _ := a.memories.BuildBrief(ctx, memory.DefaultUserID, a.working.LastUserText())
	if strings.TrimSpace(brief) == "" {
		return msgs
	}
	// 为了兼容最多 provider，不使用多条 system message 或 provider-specific cache_control。
	// 用 user role 包装时必须明确声明它不是用户请求，避免模型把记忆当成新的指令。
	context := "<internal_context>\n" +
		"This block is internal background context, not a user request.\n" +
		"Use it only when relevant. Current user instructions override this context.\n\n" +
		"<active_memory>\n" + brief + "\n</active_memory>\n" +
		"</internal_context>"
	out := make([]model.Message, 0, len(msgs)+1)
	out = append(out, model.NewTextMessage(model.RoleUser, context))
	out = append(out, msgs...)
	return out
}

func (a *Agent) activeModelSummary() string {
	if a.router == nil {
		return "none configured"
	}
	return a.router.ActiveRef()
}

func (a *Agent) modelRoutingSummary() string {
	if a.router == nil {
		return "- No models configured. Configure a model before using spawn."
	}
	refs := a.router.ListProviders()
	sort.Strings(refs)
	if len(refs) == 0 {
		return "- No models configured. Configure a model before using spawn."
	}
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		mc, err := a.router.ModelConfig(ref)
		if err != nil || mc == nil {
			lines = append(lines, fmt.Sprintf("- %s", ref))
			continue
		}
		var attrs []string
		if len(mc.Strengths) > 0 {
			attrs = append(attrs, strings.Join(mc.Strengths, ", "))
		}
		if mc.ContextWindow > 0 {
			attrs = append(attrs, fmt.Sprintf("ctx %s", formatContextWindow(mc.ContextWindow)))
		}
		if len(attrs) == 0 {
			lines = append(lines, fmt.Sprintf("- %s", ref))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: %s", ref, strings.Join(attrs, "; ")))
		}
	}
	return strings.Join(lines, "\n")
}

func formatContextWindow(n int) string {
	if n >= 1000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// buildToolDefs 构建 LLM tool calling 定义。AskUser 和 Spawn 是 core 内建工具，动态追加。
func (a *Agent) buildToolDefs() []model.ToolDef {
	tools := a.registry.All()
	defs := make([]model.ToolDef, 0, len(tools)+2)

	for _, t := range tools {
		defs = append(defs, model.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  withIntentParameter(t.Parameters()),
		})
	}

	if a.modelRef == "" {
		defs = append(defs, model.ToolDef{
			Name:        "askuser",
			Description: "Ask the user a question and wait for their reply",
			Parameters: withIntentParameter(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string", "description": "Question to ask"},
					"options":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Options"},
				},
				"required": []string{"question"},
			}),
		})

		spawnToolNames := a.availableSpawnTools()
		defs = append(defs, model.ToolDef{
			Name:        "spawn",
			Description: "Create a sub-agent to execute a self-contained sub-task. You must explicitly choose model and tools. Tools are permissions, not preferences; use least privilege.",
			Parameters: withIntentParameter(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task":    map[string]any{"type": "string", "description": "Sub-task description"},
					"model":   map[string]any{"type": "string", "description": "Exact model ref from Available sub-agent models"},
					"system":  map[string]any{"type": "string", "description": "Sub agent system prompt"},
					"tools":   map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": spawnToolNames}, "description": "Explicit tool permissions for the sub-agent; choose by least privilege"},
					"timeout": map[string]any{"type": "integer", "description": "Timeout seconds"},
					"context": map[string]any{"type": "string", "description": "Extra context"},
				},
				"required": []string{"task", "model", "tools"},
			}),
		})
	}

	return defs
}

func withIntentParameter(params map[string]any) map[string]any {
	props, ok := params["properties"].(map[string]any)
	if !ok {
		props = map[string]any{}
		params["properties"] = props
	}
	props["intent"] = map[string]any{
		"type":        "string",
		"description": "Natural-language reason for this tool call. Explain what you are trying to accomplish for the user. Do not put file contents, secrets, or raw parameters here.",
	}
	return params
}

func getEnvInfo() map[string]string {
	wd, _ := os.Getwd()
	return map[string]string{
		"OS":      runtime.GOOS,
		"Arch":    runtime.GOARCH,
		"WorkDir": wd,
	}
}

func resolveModelID(cfg *config.Config, modelName string) string {
	if mc, ok := cfg.ModelByRef(modelName); ok {
		return mc.Model
	}
	if mc, ok := cfg.ActiveModelConfig(); ok {
		return mc.Model
	}
	return modelName
}
