/* config-home.js — 配置主页渲染。
   模型优先布局：0 模型时顶部 Setup 引导卡片；通用设置折叠在下方。
   依赖 config-shared.js 的工具函数。 */

import { escapeHtml } from './core.js';
import { sourceBadgeHTML, LOCALES, THEMES, GUARD_MODES } from './config-shared.js';

// homeHTML 渲染配置主页。
export function homeHTML(view) {
  const s = view.state || {};
  const models = Array.isArray(s.models) ? s.models : [];
  const active = s.active_model || '（未激活）';
  const needs = models.filter(m => !m.has_api_key || !m.model || !m.base_url || !m.context_window || !m.max_output_tokens || m.max_output_tokens >= m.context_window).length;
  const providersSummary = models.length === 0
    ? '（无）'
    : (models.length + ' 个' + (needs ? '，' + needs + ' 个待补全' : ''));

  const wsValue = s.workspace || '';
  const sources = s.sources || {};

  return ''
    + '<div class="config-view">'
    + '  <div class="config-view-header">'
    + '    <div class="config-view-title">设置</div>'
    + '  </div>'

    + (models.length === 0
      ? '  <div class="config-setup-banner">'
        + '    <div class="config-setup-title">先添加一个模型</div>'
        + '    <div class="config-setup-hint">Suna 不会预置任何厂商或密钥，必须先选协议标准（OpenAI / Anthropic），再填端点、密钥和模型名。</div>'
        + '    <div class="config-setup-actions">'
        + '      <button class="btn btn-primary" data-act="add-model">添加第一个模型</button>'
        + '    </div>'
        + '  </div>'
      : '')

    + '  <div class="section-label">模型连接</div>'
    + '  <div class="config-row clickable" data-act="open-models">'
    + '    <span class="config-row-label">▸ 模型列表</span>'
    + '    <span class="config-row-value">' + (models.length === 0 ? '尚未添加模型' : providersSummary) + '</span>'
    + '  </div>'
    + '  <div class="config-row">'
    + '    <span class="config-row-label">当前激活</span>'
    + '    <span class="config-row-value mono">' + escapeHtml(active) + '</span>'
    + '    ' + sourceBadgeHTML(sources.active_model)
    + '  </div>'

    + '  <div class="section-label">通用</div>'
    + generalRow('language', '语言', s.locale || 'zh', LOCALES, sources.ui_locale)
    + generalRow('theme', '主题', s.theme || 'auto', THEMES, sources.ui_theme)
    + generalRow('guard', '安全模式', s.guard_mode || 'ask', GUARD_MODES, sources.guard_mode)
    + '  <div class="config-row">'
    + '    <span class="config-row-label">Workspace</span>'
    + '    <span class="config-row-value mono ellipsis" title="' + escapeHtml(wsValue) + '">' + escapeHtml(wsValue || '（未设置，将允许工作目录内任意读写）') + '</span>'
    + '    ' + sourceBadgeHTML(sources.guard_workspace)
    + '    <span class="config-row-actions">'
    + '      <button class="btn btn-ghost" data-act="edit-workspace">编辑</button>'
    + '    </span>'
    + '  </div>'

    + '  <div class="section-label">配置文件</div>'
    + '  <div class="config-row">'
    + '    <span class="config-row-label">全局配置</span>'
    + '    <span class="config-row-value mono ellipsis">' + escapeHtml(s.global_config_path || '~/.suna/config.toml') + '</span>'
    + '  </div>'
    + '  <div class="config-row">'
    + '    <span class="config-row-label">项目配置</span>'
    + '    <span class="config-row-value mono ellipsis">' + (s.project_config_path ? escapeHtml(s.project_config_path) : '未检测到 .suna/config.toml') + '</span>'
    + '  </div>'

    + '  <div class="section-label">高级设置</div>'
    + '  <div class="config-row">'
    + '    <span class="config-row-label">最大模型 RPS</span>'
    + '    <span class="config-row-value">'
    + '      <input type="number" min="0" step="1" class="input input-mono" data-act="set-max-rps" value="' + (s.max_model_rps || 0) + '" style="width:90px" />'
    + '    </span>'
    + '    ' + sourceBadgeHTML(sources.max_model_rps)
    + '  </div>'
    + '  <div class="config-row">'
    + '    <span class="config-row-label">MCP servers</span>'
    + '    <span class="config-row-value mono">' + (s.has_mcp ? '已配置（需直接编辑配置文件）' : '无') + '</span>'
    + '  </div>'
    + '  <div class="config-row">'
    + '    <span class="config-row-label">Hooks</span>'
    + '    <span class="config-row-value mono">' + (s.has_hooks ? '已配置（需直接编辑配置文件）' : '无') + '</span>'
    + '  </div>'

    + '</div>';
}

function generalRow(kind, label, current, options, source) {
  const opts = options.map(o =>
    '<option value="' + escapeHtml(o.value) + '"' + (o.value === current ? ' selected' : '') + '>' + escapeHtml(o.label) + '</option>'
  ).join('');
  return ''
    + '<div class="config-row">'
    + '  <span class="config-row-label">' + escapeHtml(label) + '</span>'
    + '  <span class="config-row-value">'
    + '    <select class="config-select" data-act="set-general" data-kind="' + escapeHtml(kind) + '">' + opts + '</select>'
    + '  </span>'
    + '  ' + sourceBadgeHTML(source)
    + '</div>';
}
