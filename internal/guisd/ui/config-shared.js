/* config-shared.js — 配置面板共享工具函数和常量。
   被 config-home.js / config-models.js / config-detail.js 共同依赖。 */

import { escapeHtml } from './core.js';

// 通用设置的可选值常量。
export const LOCALES = [
  { value: 'zh', label: '中文' },
  { value: 'en', label: 'English' },
];
export const THEMES = [
  { value: 'auto', label: '自动' },
  { value: 'dark', label: '深色' },
  { value: 'light', label: '浅色' },
];
export const GUARD_MODES = [
  { value: 'ask', label: '每次询问（默认）' },
  { value: 'smart', label: '智能（高风险才问）' },
  { value: 'auto', label: '全自动' },
  { value: 'readonly', label: '只读（禁止写操作）' },
];

// sourceBadgeHTML 渲染"全局 / 项目"来源徽标；source 为空时不渲染。
export function sourceBadgeHTML(source) {
  if (!source) return '';
  if (source === 'project') return '<span class="badge badge-accent" title="值来自项目级 .suna/config.toml">项目</span>';
  return '<span class="badge badge-neutral" title="值来自全局 ~/.suna/config.toml">全局</span>';
}

// joinRef 构造模型 ref：provider/model。
export function joinRef(provider, model) {
  return provider + '/' + model;
}

// fmtInt 格式化整数，0 或无效值显示 ——。
export function fmtInt(n) {
  if (!n || n <= 0) return '—';
  return String(n);
}

// isModelNeedsAttention 判断模型是否缺少关键字段。
export function isModelNeedsAttention(m) {
  return !m.has_api_key || !m.model || !m.base_url || !m.context_window || !m.max_output_tokens || m.max_output_tokens >= m.context_window;
}

// modelStatusMark 返回模型状态标记：! 需补全、◉ 已激活、○ 未激活。
export function modelStatusMark(m, active) {
  if (isModelNeedsAttention(m)) return '!';
  if (active) return '◉';
  return '○';
}

// modelSummary 生成模型摘要文本。
export function modelSummary(m) {
  const parts = [];
  if (!m.has_api_key) parts.push('缺少 API Key');
  if (m.context_window) parts.push('ctx ' + m.context_window);
  if (m.max_output_tokens) parts.push('out ' + m.max_output_tokens);
  if (Array.isArray(m.strengths) && m.strengths.length) parts.push(m.strengths.join(', '));
  return parts.join(' · ');
}

// scopeSwitcherHTML 渲染顶部"全局 / 项目"切换按钮组。
export function scopeSwitcherHTML(view) {
  const hasProject = !!(view.state && view.state.project_config_path);
  if (!hasProject) {
    return '<div class="config-scope-wrap"><div class="config-scope-info">作用域：仅全局（当前目录无 <code>.suna/config.toml</code>）</div></div>';
  }
  const g = view.scope === 'global' ? ' active' : '';
  const p = view.scope === 'project' ? ' active' : '';
  return ''
    + '<div class="config-scope-wrap">'
    + '<div class="config-scope-switcher" role="tablist" aria-label="配置作用域">'
    + '  <button class="config-scope-btn' + g + '" data-act="set-scope" data-scope="global" role="tab" aria-selected="' + (view.scope === 'global') + '">全局</button>'
    + '  <button class="config-scope-btn' + p + '" data-act="set-scope" data-scope="project" role="tab" aria-selected="' + (view.scope === 'project') + '">项目</button>'
    + '</div>'
    + (view.scope === 'project' ? '<div class="config-scope-hint">改动会写入 <code>.suna/config.toml</code>。该文件含项目专属配置，建议加入 <code>.gitignore</code>。</div>' : '')
    + '</div>';
}

// modelByRef 在 view.state.models 中按 ref 查找模型。
export function modelByRef(view, ref) {
  if (!view.state || !Array.isArray(view.state.models)) return null;
  for (const m of view.state.models) {
    if (joinRef(m.provider, m.model) === ref) {
      return Object.assign({}, m, { ref: joinRef(m.provider, m.model) });
    }
  }
  return null;
}
