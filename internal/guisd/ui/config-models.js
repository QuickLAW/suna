/* config-models.js — 模型列表页渲染。
   依赖 config-shared.js 的工具函数。 */

import { escapeHtml } from './core.js';
import { joinRef, modelStatusMark, modelSummary, scopeSwitcherHTML, sourceBadgeHTML } from './config-shared.js';

// modelsListHTML 渲染模型列表页。
export function modelsListHTML(view) {
  const s = view.state || {};
  const models = Array.isArray(s.models) ? s.models.slice() : [];
  models.sort((a, b) => joinRef(a.provider, a.model).localeCompare(joinRef(b.provider, b.model)));
  const active = s.active_model || '';

  let body;
  if (models.length === 0) {
    body = '<div class="config-empty">尚未添加模型。<br>点击下方"添加第一个模型"开始。</div>';
  } else {
    body = models.map(m => {
      const ref = joinRef(m.provider, m.model);
      const mark = modelStatusMark(m, ref === active);
      const isActive = ref === active;
      return ''
        + '<div class="config-model-row" data-act="open-detail" data-ref="' + escapeHtml(ref) + '">'
        + '  <span class="cmr-mark">' + mark + '</span>'
        + '  <span class="cmr-ref mono">' + escapeHtml(ref) + (isActive ? ' <span class="badge badge-success">已激活</span>' : '') + '</span>'
        + '  <span class="cmr-summary">' + escapeHtml(modelSummary(m)) + '</span>'
        + '</div>';
    }).join('');
  }

  return ''
    + '<div class="config-view">'
    + scopeSwitcherHTML(view)
    + '  <div class="config-view-header">'
    + '    <div class="config-view-title">模型列表</div>'
    + '    <button class="btn btn-ghost" data-act="go-home">‹ 返回</button>'
    + '  </div>'
    + body
    + '  <div class="config-form-actions">'
    + '    <button class="btn btn-primary" data-act="add-model">' + (models.length === 0 ? '添加第一个模型' : '+ 添加模型') + '</button>'
    + '  </div>'
    + '</div>';
}
