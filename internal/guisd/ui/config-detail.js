/* config-detail.js — 模型详情页渲染。
   展示单个模型的完整信息 + 编辑/删除/激活操作。
   依赖 config-shared.js 的工具函数。 */

import { escapeHtml } from './core.js';
import { joinRef, fmtInt, modelStatusMark, modelByRef, scopeSwitcherHTML, sourceBadgeHTML } from './config-shared.js';
import { matchReasoningKey } from './config-form.js';

// detailHTML 渲染模型详情页。
export function detailHTML(view, form) {
  const m = modelByRef(view, view.detailRef);
  if (!m) return null; // 外层 fallback 到 models 列表
  const active = view.state && view.state.active_model === view.detailRef;
  const apiKeyLabel = m.has_api_key ? '已配置' : '缺失';
  const status = active ? '已激活' : '未激活';
  const sources = (view.state || {}).sources || {};

  return ''
    + '<div class="config-view">'
    + scopeSwitcherHTML(view)
    + '  <div class="config-view-header">'
    + '    <div class="config-view-title">模型：' + escapeHtml(view.detailRef) + '</div>'
    + '    <button class="btn btn-ghost" data-act="go-models">‹ 返回模型列表</button>'
    + '  </div>'

    + '  <div class="section-label">概要</div>'
    + infoRow('状态', status)
    + infoRow('Provider', m.provider, sources.models)
    + infoRow('Endpoint', m.base_url || '—', sources.models)
    + infoRow('API Key', apiKeyLabel, sources.models)
    + infoRow('Model', modelStatusMark(m, active) + ' ' + m.model, sources.models)
    + infoRow('Context Window', fmtInt(m.context_window), sources.models)
    + infoRow('Max Output Tokens', fmtInt(m.max_output_tokens), sources.models)
    + infoRow('Reasoning', reasoningLabel(form, m) || '—', sources.models)
    + infoRow('Subtask For', (Array.isArray(m.subtask_for) && m.subtask_for.length) ? m.subtask_for.join(', ') : '全部（未限定）')

    + '  <div class="section-label">操作</div>'
    + '  <div class="config-form-actions">'
    + '    <button class="btn btn-ghost" data-act="edit-provider" data-ref="' + escapeHtml(view.detailRef) + '">编辑连接</button>'
    + '    <button class="btn btn-ghost" data-act="edit-reasoning" data-ref="' + escapeHtml(view.detailRef) + '">编辑 Reasoning</button>'
    +    (active ? '' : '<button class="btn btn-primary" data-act="activate-model" data-ref="' + escapeHtml(view.detailRef) + '">激活该模型</button>')
    + '    <button class="btn btn-danger" data-act="begin-delete" data-ref="' + escapeHtml(view.detailRef) + '">删除</button>'
    + '  </div>'

    + (view.deleteConfirm === view.detailRef ? deleteConfirmHTML(view, m) : '')
    + '</div>';
}

function infoRow(k, v, source) {
  return '<div class="config-row"><span class="config-row-label">' + escapeHtml(k) + '</span><span class="config-row-value mono">' + escapeHtml(v) + '</span>' + sourceBadgeHTML(source) + '</div>';
}

function reasoningLabel(form, m) {
  if (!m || !m.reasoning) return '';
  const k = form.matchReasoningKey(m);
  if (k) {
    const [fam, label] = k.split(':');
    return fam + ' / ' + label;
  }
  return '自定义 JSON';
}

function deleteConfirmHTML(view, m) {
  const models = (view.state || {}).models || [];
  const isLastProvider = !models.some(x => x !== m && x.provider === m.provider);
  return ''
    + '<div class="config-confirm">'
    + '  <div class="config-confirm-title">确认删除 <code>' + escapeHtml(view.detailRef) + '</code>？</div>'
    +    (isLastProvider && m.has_api_key
      ? '<div class="config-confirm-hint">这是该 Provider 的最后一个模型，删除时可以选择同时清除其 API Key 凭证。</div>'
      : '')
    + '  <div class="config-form-actions">'
    + '    <button class="btn btn-ghost" data-act="cancel-delete">取消</button>'
    + '    <button class="btn btn-danger" data-act="confirm-delete">仅删除模型</button>'
    +    (isLastProvider && m.has_api_key
      ? '<button class="btn btn-primary btn-danger" data-act="confirm-delete-with-key">删除并清除 API Key</button>'
      : '')
    + '  </div>'
    + '</div>';
}
