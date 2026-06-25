/* config-form.js — 配置面板的表单（Provider / Reasoning）渲染。
   通过工厂 createConfigForm(deps) 接收 config.js 注入的依赖：
   - getView: () => view   // config.js 维护的视图状态
   - render: () => void    // 重新渲染配置面板
   - setFormError: (msg)   // 写入错误
   - send(method, params)  // JSON-RPC 通道
   - joinRef / modelByRef / escapeHtml
   依赖 core.js 的 modal（自定义 JSON 弹窗）。 */

// Provider 表单的字段定义。
// - provider: 渲染为只读 kind 标签 + "重新选择"按钮；值由 pickProviderKind 模态选择后注入。
// - model: 输入框 + Fetch 按钮（调 config.list_models 拉取供应商模型列表）。
// - apiKey: 密码输入框；编辑已有 key 的模型时占位符提示"已配置 · 留空保持不变"。
// 注意：新增 provider 类型（如未来支持 Cohere 等）时，只需在此处加 def 即可，
// pickProviderKind 模态和 doFetchProviderModels 都从这里读类型。
const PROVIDER_FIELD_DEFS = [
  { key: 'provider', label: 'Provider（协议标准）', type: 'provider-kind', placeholder: 'openai / anthropic' },
  { key: 'model', label: 'Model', type: 'model-with-fetch', placeholder: 'gpt-4o-mini' },
  { key: 'apiKey', label: 'API Key', type: 'password', placeholder: 'sk-...' },
  { key: 'endpoint', label: 'Endpoint (Base URL)', type: 'text', placeholder: 'https://api.example.com/v1' },
  { key: 'contextWindow', label: 'Context Window', type: 'number', placeholder: '128000' },
  { key: 'maxOutputTokens', label: 'Max Output Tokens', type: 'number', placeholder: '8192' },
  { key: 'strengths', label: 'Strengths（逗号分隔）', type: 'text', placeholder: 'code, reasoning' },
  { key: 'subtaskFor', label: 'Subtask For（逗号分隔）', type: 'text', placeholder: 'code-review, summarize' },
];

// PROVIDER_FIELD_DEFS_BY_KEY 方便外部按 key 查 def。
const PROVIDER_FIELD_DEFS_BY_KEY = PROVIDER_FIELD_DEFS.reduce((m, f) => { m[f.key] = f; return m; }, {});

// Reasoning 预设：与 TUI 的 ReasoningOptions 严格对齐。
const REASONING_PRESETS = [
  { family: 'GPT', options: [
    { label: 'None', key: 'gpt:None' },
    { label: 'Minimal', key: 'gpt:Minimal' },
    { label: 'Low', key: 'gpt:Low' },
    { label: 'Medium', key: 'gpt:Medium' },
    { label: 'High', key: 'gpt:High' },
    { label: 'XHigh', key: 'gpt:XHigh' },
  ]},
  { family: 'Claude', options: [
    { label: 'Fast (1024)', key: 'claude:Fast' },
    { label: 'Balanced (2048)', key: 'claude:Balanced' },
    { label: 'Deep (3072)', key: 'claude:Deep' },
  ]},
  { family: 'DeepSeek V4', options: [
    { label: 'Disabled', key: 'deepseek:Disabled' },
    { label: 'High', key: 'deepseek:High' },
    { label: 'Max', key: 'deepseek:Max' },
  ]},
  { family: 'MiniMax M3', options: [
    { label: 'Split', key: 'minimax:Split' },
  ]},
];

// ============= 工具函数 =============

// validateProviderForm 校验 Provider 表单字段，返回错误消息或空字符串。
export function validateProviderForm(v) {
  if (!v.provider || !v.provider.trim()) return 'Provider 不能为空';
  if (!v.model || !v.model.trim()) return 'Model 不能为空';
  if (!v.endpoint || !v.endpoint.trim()) return 'Endpoint (Base URL) 不能为空';
  try {
    const u = new URL(v.endpoint.trim());
    if (!u.protocol || !u.host) return 'Endpoint 不是合法 URL';
  } catch {
    return 'Endpoint 不是合法 URL';
  }
  const ctx = parsePositiveInt(v.contextWindow);
  if (!ctx) return 'Context Window 必须是正整数';
  const out = parsePositiveInt(v.maxOutputTokens);
  if (!out) return 'Max Output Tokens 必须是正整数';
  if (out >= ctx) return 'Max Output Tokens 必须小于 Context Window';
  return '';
}

export function parsePositiveInt(s) {
  const n = parseInt((s || '').trim(), 10);
  return Number.isFinite(n) && n > 0 ? n : 0;
}

export function splitCSV(s) {
  return (s || '').split(',').map(x => x.trim()).filter(Boolean);
}

// matchReasoningKey 推断当前 ModelConfig.Reasoning 与预设的匹配 key。
// 返回 'family:Label' 或 null（表示自定义 JSON）。
export function matchReasoningKey(m) {
  if (!m || !m.reasoning) return null;
  const r = m.reasoning;
  if (r.reasoning && r.reasoning.effort) {
    const e = String(r.reasoning.effort);
    const found = REASONING_PRESETS[0].options.find(o => o.label.toLowerCase() === e.toLowerCase());
    if (found) return found.key;
  }
  if (typeof r.reasoning_effort === 'string') {
    const e = r.reasoning_effort;
    const found = REASONING_PRESETS[0].options.find(o => o.label.toLowerCase() === e.toLowerCase());
    if (found) return found.key;
  }
  if (r.thinking && r.thinking.type === 'enabled' && typeof r.thinking.budget_tokens === 'number') {
    const t = r.thinking.budget_tokens;
    const found = REASONING_PRESETS[1].options.find(o => o.label.startsWith('Fast') && t === 1024)
      || REASONING_PRESETS[1].options.find(o => o.label.startsWith('Balanced') && t === 2048)
      || REASONING_PRESETS[1].options.find(o => o.label.startsWith('Deep') && t === 3072);
    if (found) return found.key;
  }
  if (r.thinking && r.thinking.type === 'disabled') return 'deepseek:Disabled';
  if (r.thinking && r.thinking.type === 'enabled' && typeof r.reasoning_effort === 'string') {
    const e = r.reasoning_effort.toLowerCase();
    const found = REASONING_PRESETS[2].options.find(o => o.label.toLowerCase() === e);
    if (found) return found.key;
  }
  if (r.reasoning_split === true) return 'minimax:Split';
  return null;
}

// buildReasoningFor 根据 family:Label key 构造 Reasoning 配置对象。
// 返回 null 表示清除（无 Reasoning）。
function buildReasoningFor(key) {
  if (!key) return null;
  const [family, label] = key.split(':');
  switch (family) {
    case 'GPT':
      return { reasoning: { effort: label.toLowerCase() } };
    case 'Claude': {
      const t = label.startsWith('Fast') ? 1024 : label.startsWith('Balanced') ? 2048 : 3072;
      return { thinking: { type: 'enabled', budget_tokens: t } };
    }
    case 'DeepSeek': {
      if (label === 'Disabled') return { thinking: { type: 'disabled' } };
      return { thinking: { type: 'enabled' }, reasoning_effort: label.toLowerCase() };
    }
    case 'MiniMax':
      return { reasoning_split: true };
  }
  return null;
}

// ============= 工厂 =============

export function createConfigForm(deps) {
  const { getView, render, setFormError, send, joinRef, modelByRef, escapeHtml, modal } = deps;

  // openExistingForm 把当前 view.formMode 的表单重新渲染。
  // setFormError 在表单校验失败时调用，避免 onInput 重复触发整表重渲。
  function openExistingForm() {
    const view = getView();
    if (view.formMode === 'provider') renderProviderForm();
    else if (view.formMode === 'reasoning') renderReasoningForm();
  }

  // openProviderForm 切换 view 状态为新增 / 编辑 Provider 表单。
  // 新增模式下 formValues 留空，不再预填 "openai-compatible"。
  // 新增时先由 pickProviderKind() 让用户选 openai/anthropic，再通过 setDefaultProviderKind
  // 把选中的 kind 注入 formValues.provider，让用户不必手填。
  function openProviderForm(ref) {
    const view = getView();
    const m = ref ? modelByRef(ref) : null;
    view.formMode = 'provider';
    view.formEditing = ref || '';
    view.formValues = m ? {
      provider: m.provider || '',
      model: m.model || '',
      apiKey: '',
      endpoint: m.base_url || '',
      contextWindow: m.context_window ? String(m.context_window) : '',
      maxOutputTokens: m.max_output_tokens ? String(m.max_output_tokens) : '',
      strengths: Array.isArray(m.strengths) ? m.strengths.join(', ') : '',
      subtaskFor: Array.isArray(m.subtask_for) ? m.subtask_for.join(', ') : '',
    } : {
      provider: '', model: '', apiKey: '', endpoint: '',
      contextWindow: '', maxOutputTokens: '', strengths: '', subtaskFor: '',
    };
    view.formError = '';
    renderProviderForm();
  }

  // setDefaultProviderKind 在 pickProviderKind 模态选中后调用，
  // 把选中的 openai / anthropic 写入当前 formValues.provider 并触发重渲。
  // 编辑模式不应调用（编辑时 kind 由 ref 决定，不可改）。
  function setDefaultProviderKind(kind) {
    const view = getView();
    if (!view.formValues) return;
    view.formValues.provider = kind || '';
    renderProviderForm();
  }

  function renderProviderForm() {
    const view = getView();
    const body = document.getElementById('editorBody');
    if (!body) return;
    const v = view.formValues;
    const title = view.formEditing ? '编辑模型：' + view.formEditing : '添加模型';
    // 字段渲染分三种特殊类型：provider-kind（只读标签 + 重新选）、model-with-fetch（输入框 + Fetch 按钮）、password（动态 placeholder）。
    // 其余 text / number 走通用 input。
    const fields = PROVIDER_FIELD_DEFS.map(f => renderProviderField(f, v, view)).join('');

    body.innerHTML = ''
      + '<div class="config-view">'
      + '  <div class="config-view-header">'
      + '    <div class="config-view-title">' + escapeHtml(title) + '</div>'
      + '    <button class="chat-btn ghost" data-act="cancel-form">取消</button>'
      + '  </div>'
      + '  <div class="config-form-grid">' + fields + '</div>'
      + '  <div class="config-form-actions">'
      + '    <button class="chat-btn primary" data-act="save-provider">' + (view.formEditing ? '保存修改' : '添加') + '</button>'
      + '  </div>'
      + (view.formError ? '<div class="config-form-error">' + escapeHtml(view.formError) + '</div>' : '')
      + '</div>';
    body.scrollTop = 0;
  }

  // renderProviderField 按字段 type 渲染对应 HTML。
  // - provider-kind: 显示当前选中的 kind + 重新选按钮；不写 data-field 阻止 onFieldInput 处理。
  // - model-with-fetch: 输入框 + 旁边 Fetch 按钮（触发 doFetchProviderModels）。
  // - password: 编辑已有 key 时占位符提示"已配置 · 留空保持不变"，新增时显示 sk-...。
  // - 其它: 通用 input。
  function renderProviderField(f, v, view) {
    const label = '<label class="config-form-label">' + escapeHtml(f.label) + '</label>';
    const val = v[f.key] || '';
    if (f.type === 'provider-kind') {
      const cur = (val || '').trim();
      const tag = cur
        ? '<span class="config-form-kind-tag">' + escapeHtml(cur) + '</span>'
        : '<span class="config-form-kind-tag dim">未选择</span>';
      // 编辑模式隐藏"重新选择"按钮（kind 由 ref 决定，不可改）。
      const rePick = view.formEditing
        ? ''
        : ' <button class="chat-btn ghost" data-act="repick-provider-kind" type="button">重新选择</button>';
      return '<div class="config-form-field">'
        + label
        + '<div class="config-form-kind-row">' + tag + rePick + '</div>'
        + '</div>';
    }
    if (f.type === 'model-with-fetch') {
      return '<div class="config-form-field">'
        + label
        + '<div class="config-form-model-row">'
        + '  <input class="config-form-input" data-field="model" type="text" value="' + escapeHtml(val) + '" placeholder="' + escapeHtml(f.placeholder || '') + '" autocomplete="off" />'
        + '  <button class="chat-btn primary" data-act="fetch-provider-models" type="button">Fetch</button>'
        + '</div>'
        + '<div class="config-form-hint">点击 Fetch 会用当前 Endpoint + API Key 调供应商的 /v1/models；选中后自动回填。</div>'
        + '</div>';
    }
    if (f.type === 'password') {
      // 编辑已有 key 的模型时显示"已配置 · 留空保持不变"，新增时显示 sk-...
      const hasKey = view.formEditing && (modelByRef(view.formEditing) || {}).has_api_key;
      const placeholder = hasKey ? '已配置 · 留空保持不变' : (f.placeholder || 'sk-...');
      return '<div class="config-form-field">'
        + label
        + '<input class="config-form-input" data-field="apiKey" type="password" value="" placeholder="' + escapeHtml(placeholder) + '" autocomplete="off" />'
        + '</div>';
    }
    return '<div class="config-form-field">'
      + label
      + '<input class="config-form-input" data-field="' + escapeHtml(f.key) + '" type="' + escapeHtml(f.type) + '" value="' + escapeHtml(val) + '" placeholder="' + escapeHtml(f.placeholder || '') + '" autocomplete="off" />'
      + '</div>';
  }

  // doFetchProviderModels 用当前表单的 endpoint + apiKey 调 config.list_models，
  // 弹 pickFromList 模态让用户选模型；选中后回填到 model 字段。
  // 注意：apiKey 在编辑已有 key 时为空，这时服务端会要求前端用现有 key；
  // 后端 ConfigListModels 不接受"不覆盖"的语义，必须传实际 key 值，所以编辑模式下
  // apiKey 字段会被临时提示用户"留空"时无法 fetch，需用户重新输入。
  async function doFetchProviderModels() {
    const view = getView();
    const v = view.formValues || {};
    const provider = (v.provider || '').trim();
    if (!provider) { setFormError('请先选择 Provider 协议标准'); return; }
    const endpoint = (v.endpoint || '').trim();
    if (!endpoint) { setFormError('请先填写 Endpoint'); return; }
    const apiKey = (v.apiKey || '').trim();
    if (!apiKey) {
      // 编辑模式且未填 key：给出明确提示
      const m = view.formEditing ? modelByRef(view.formEditing) : null;
      if (m && m.has_api_key) {
        setFormError('已配置了 API Key，但前端无法读取明文，请重新输入后点击 Fetch');
        return;
      }
      setFormError('请先填写 API Key');
      return;
    }
    setFormError('');
    let result;
    try {
      result = await send('config.list_models', { provider, base_url: endpoint, api_key: apiKey });
    } catch (err) {
      setFormError('拉取模型失败：' + (err && err.message ? err.message : String(err)));
      return;
    }
    if (!result) { setFormError('拉取模型失败：未返回结果'); return; }
    if (result.error) { setFormError('拉取模型失败：' + result.error); return; }
    const models = Array.isArray(result.models) ? result.models : [];
    if (models.length === 0) { setFormError('该供应商未返回任何模型'); return; }
    const picked = await modal.pickFromList({
      title: '选择模型（' + provider + '）',
      message: 'Endpoint: ' + endpoint,
      options: models.map((id) => ({ value: id, label: id })),
      emptyText: '供应商返回了 0 个模型',
    });
    if (picked) {
      v.model = picked;
      renderProviderForm();
    }
  }

  // openReasoningForm 切换 view 状态为编辑 Reasoning 二级菜单。
  function openReasoningForm(ref) {
    const view = getView();
    if (!modelByRef(ref)) return;
    view.formMode = 'reasoning';
    view.formEditing = ref;
    view.reasonMenu = null;
    view.formError = '';
    renderReasoningForm();
  }

  function renderReasoningForm() {
    const view = getView();
    const body = document.getElementById('editorBody');
    if (!body) return;
    const m = modelByRef(view.formEditing);
    if (!m) { view.formMode = null; render(); return; }
    const current = matchReasoningKey(m);
    if (!view.reasonMenu) {
      const items = [
        { key: '__clear__', label: '清除（无 Reasoning）' },
        ...REASONING_PRESETS.map(p => ({ key: '__family__:' + p.family, label: p.family })),
        { key: '__custom__', label: '自定义 JSON' },
      ];
      const list = items.map(it => {
        const isCurrent = (it.key === '__clear__' && !current) || (it.key === '__custom__' && current === null && m.reasoning);
        return ''
          + '<div class="config-model-row" data-act="reason-pick" data-key="' + escapeHtml(it.key) + '">'
          + '  <span class="cmr-ref">' + escapeHtml(it.label) + (isCurrent ? ' <span class="cmr-active">当前</span>' : '') + '</span>'
          + '  <span class="cmr-summary">›</span>'
          + '</div>';
      }).join('');
      body.innerHTML = ''
        + '<div class="config-view">'
        + '  <div class="config-view-header">'
        + '    <div class="config-view-title">Reasoning：' + escapeHtml(view.formEditing) + '</div>'
        + '    <button class="chat-btn ghost" data-act="cancel-form">取消</button>'
        + '  </div>'
        + list
        + '</div>';
    } else {
      const fam = view.reasonMenu;
      const preset = REASONING_PRESETS.find(p => p.family === fam);
      if (!preset) { view.reasonMenu = null; return renderReasoningForm(); }
      const list = preset.options.map(opt => {
        const isCurrent = current === opt.key;
        return ''
          + '<div class="config-model-row" data-act="reason-apply-preset" data-key="' + escapeHtml(opt.key) + '">'
          + '  <span class="cmr-ref">' + escapeHtml(opt.label) + (isCurrent ? ' <span class="cmr-active">当前</span>' : '') + '</span>'
          + '  <span class="cmr-summary">›</span>'
          + '</div>';
      }).join('');
      body.innerHTML = ''
        + '<div class="config-view">'
        + '  <div class="config-view-header">'
        + '    <div class="config-view-title">Reasoning · ' + escapeHtml(fam) + '</div>'
        + '    <button class="chat-btn ghost" data-act="reason-back">‹ 返回</button>'
        + '  </div>'
        + list
        + '</div>';
    }
    body.scrollTop = 0;
  }

  // onReasonPick 处理 Reasoning 顶层选择：清除 / 进入 family / 自定义 JSON。
  async function onReasonPick(key) {
    const view = getView();
    if (key === '__clear__') {
      doApplyReasoning(null);
    } else if (key === '__custom__') {
      const m = modelByRef(view.formEditing);
      if (!m) return;
      const cur = JSON.stringify(m.reasoning || {}, null, 2);
      const next = await modal.prompt({
        title: '自定义 Reasoning JSON',
        message: '必须是合法 JSON 对象',
        defaultValue: cur,
        confirmText: '保存',
      });
      if (next === null) return;
      let parsed;
      try {
        parsed = JSON.parse(next || '{}');
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) throw new Error('必须是 JSON 对象');
      } catch (e) {
        setFormError('JSON 解析失败：' + e.message);
        return;
      }
      doApplyReasoning(parsed);
    } else if (key.startsWith('__family__:')) {
      view.reasonMenu = key.slice('__family__:'.length);
      renderReasoningForm();
    }
  }

  // onReasonApplyPreset 处理 Reasoning 二级选择：直接落库。
  function onReasonApplyPreset(key) {
    doApplyReasoning(buildReasoningFor(key));
  }

  function doApplyReasoning(reasoning) {
    const view = getView();
    const m = modelByRef(view.formEditing);
    if (!m) return;
    const params = {
      action: 'upsert_model',
      model_ref: view.formEditing,
      model: {
        provider: m.provider,
        model: m.model,
        base_url: m.base_url,
        context_window: m.context_window,
        max_output_tokens: m.max_output_tokens,
        strengths: m.strengths,
        subtask_for: m.subtask_for,
        reasoning: reasoning,
      },
    };
    send('config.set', params);
    view.formMode = null;
    view.reasonMenu = null;
    view.formError = '';
    render();
  }

  return {
    openProviderForm,
    setDefaultProviderKind,
    doFetchProviderModels,
    openReasoningForm,
    openExistingForm,
    validateProviderForm,
    parsePositiveInt,
    splitCSV,
    matchReasoningKey,
    onReasonPick,
    onReasonApplyPreset,
  };
}
