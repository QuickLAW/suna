/* config.js — 配置面板协调层。
   职责：路由（home/models/detail）、事件代理、表单操作、写操作桥接。
   渲染委托给 config-home.js / config-models.js / config-detail.js。
   表单委托给 config-form.js。
   依赖 core.js / config-shared.js / config-home.js / config-models.js / config-detail.js / config-form.js。 */

import { $, escapeHtml, modal, state } from './core.js';
import { joinRef, modelByRef } from './config-shared.js';
import { homeHTML } from './config-home.js';
import { modelsListHTML } from './config-models.js';
import { detailHTML } from './config-detail.js';
import { createConfigForm } from './config-form.js';

// ============= 视图状态 =============

const view = {
  state: { project_config_path: '', sources: {} },
  page: 'home',
  detailRef: '',
  scope: 'global',
  formMode: null,
  formEditing: '',
  formValues: {},
  formError: '',
  deleteConfirm: null,
  deleteActiveAs: '',
  reasonMenu: null,
  deleteBusy: false,
};

// ============= 桥接 =============

let _agentSend = null;
export function bindAgentSend(fn) { _agentSend = fn; }
function send(method, params) {
  if (!_agentSend) return null;
  return _agentSend(method, params);
}

function sendConfigSet(params) {
  return send('config.set', Object.assign({ scope: view.scope }, params));
}

// ============= 顶层渲染 =============

// _eventsBound 标记事件代理是否已绑定；bindConfigEvents 只在第一次调用时注册，
// 后续 renderConfigView 只更新 innerHTML，不再重复注册监听器。
let _eventsBound = false;

export function renderConfigView(snapshot) {
  if (snapshot) view.state = snapshot;
  const body = $('editorBody');
  if (!body) return;
  if (view.formMode) {
    form.openExistingForm();
    return;
  }
  let html;
  if (view.page === 'home') html = homeHTML(view);
  else if (view.page === 'models') html = modelsListHTML(view);
  else if (view.page === 'detail') {
    html = detailHTML(view, form);
    if (html === null) {
      view.page = 'models';
      html = modelsListHTML(view);
    }
  }
  body.innerHTML = html;
  body.scrollTop = 0;
  bindConfigEvents();
  if (view.formError) {
    const err = body.querySelector('.config-form-error');
    if (!err) {
      const div = document.createElement('div');
      div.className = 'config-form-error';
      div.textContent = view.formError;
      body.appendChild(div);
    }
  }
}

export function updateState(snapshot) {
  view.state = snapshot;
  if (document.body.dataset.editorMode === 'config') {
    renderConfigView();
  }
}

// ============= 事件代理 =============

function bindConfigEvents() {
  if (_eventsBound) return;
  _eventsBound = true;
  const body = $('editorBody');
  if (!body) return;
  body.addEventListener('click', onActClick);
  body.addEventListener('change', onGeneralChange);
  body.addEventListener('input', onFieldInput);
  body.addEventListener('keydown', onFieldKey);
}

function onActClick(e) {
  const el = e.target.closest('[data-act]');
  if (!el) return;
  const act = el.dataset.act;
  const ref = el.dataset.ref;
  switch (act) {
    case 'set-scope':
      view.scope = (el.dataset.scope === 'project') ? 'project' : 'global';
      renderConfigView();
      break;
    case 'open-models': view.page = 'models'; renderConfigView(); break;
    case 'go-home': view.page = 'home'; renderConfigView(); break;
    case 'go-models': view.page = 'models'; renderConfigView(); break;
    case 'add-model':
      pickProviderKind().then((kind) => {
        if (!kind) return;
        form.openProviderForm('');
        form.setDefaultProviderKind(kind);
        renderConfigView();
      });
      break;
    case 'open-detail': view.page = 'detail'; view.detailRef = ref; renderConfigView(); break;
    case 'edit-provider': form.openProviderForm(ref); break;
    case 'edit-reasoning': form.openReasoningForm(ref); break;
    case 'activate-model': doActivate(ref); break;
    case 'begin-delete': beginDelete(ref); break;
    case 'cancel-delete': view.deleteConfirm = null; renderConfigView(); break;
    case 'confirm-delete': doDelete(ref, false); break;
    case 'confirm-delete-with-key': doDelete(ref, true); break;
    case 'edit-workspace': openWorkspaceForm(); break;
    case 'cancel-form': view.formMode = null; view.formError = ''; renderConfigView(); break;
    case 'save-provider': doSaveProvider(); break;
    case 'repick-provider-kind':
      pickProviderKind().then((kind) => {
        if (kind) form.setDefaultProviderKind(kind);
      });
      break;
    case 'fetch-provider-models':
      form.doFetchProviderModels();
      break;
    case 'reason-pick': form.onReasonPick(el.dataset.key); break;
    case 'reason-apply-preset': form.onReasonApplyPreset(el.dataset.key); break;
    case 'reason-back': view.reasonMenu = null; form.openExistingForm(); break;
  }
}

function onGeneralChange(e) {
  if (view.deleteBusy) return;
  if (e.target.dataset.act === 'set-general') {
    const kind = e.target.dataset.kind;
    const value = e.target.value;
    const params = { action: 'update_general' };
    if (kind === 'language') params.locale = value;
    else if (kind === 'theme') params.theme = value;
    else if (kind === 'guard') params.guard_mode = value;
    sendConfigSet(params);
  } else if (e.target.dataset.act === 'set-max-rps') {
    const v = parseInt(e.target.value, 10);
    if (Number.isNaN(v) || v < 0) {
      view.formError = '最大 RPS 必须是非负整数';
      renderConfigView();
      return;
    }
    view.formError = '';
    sendConfigSet({ action: 'update_general', max_model_rps: v });
  }
}

function onFieldInput(e) {
  if (!e.target.dataset.field) return;
  view.formValues[e.target.dataset.field] = e.target.value;
}

function onFieldKey(e) {
  if (!e.target.dataset.field) return;
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault();
    doSaveProvider();
  } else if (e.key === 'Escape') {
    e.preventDefault();
    view.formMode = null;
    renderConfigView();
  }
}

// ============= 操作 =============

function setFormError(msg) {
  view.formError = msg || '';
  if (view.formMode) form.openExistingForm();
}

async function openWorkspaceForm() {
  const s = view.state || {};
  const cur = s.workspace || (state.workspaceRoot || '');
  const hint = state.workspaceRoot
    ? '默认 = daemon 启动工作目录 (' + state.workspaceRoot + ')'
    : '绝对路径或 ~ 开头；留空表示不限制';
  const next = await modal.prompt({
    title: '编辑 Workspace',
    message: hint,
    defaultValue: cur,
    confirmText: '保存',
  });
  if (next === null) return;
  sendConfigSet({ action: 'update_general', workspace: next.trim() === '' ? '' : next.trim() });
}

function doActivate(ref) {
  if (!ref) return;
  sendConfigSet({ action: 'activate_model', active_model: ref });
}

async function beginDelete(ref) {
  const m = modelByRef(view, ref);
  if (!m) return;
  if (m.ref === (view.state || {}).active_model) {
    const remaining = (view.state.models || []).filter(x => joinRef(x.provider, x.model) !== ref);
    if (remaining.length > 0) {
      const opts = remaining.map(x => escapeHtml(joinRef(x.provider, x.model)) + '  (' + escapeHtml(x.provider) + ')').join('\n');
      const next = await modal.prompt({
        title: '删除当前激活模型',
        message: '该模型是当前激活。删除后切到哪个？\n留空 = 自动切到 ' + joinRef(remaining[0].provider, remaining[0].model) + '。\n可选：\n' + opts,
        defaultValue: '',
        confirmText: '确认删除',
      });
      if (next === null) return;
      view.deleteActiveAs = next.trim();
    }
  }
  view.deleteConfirm = ref;
  renderConfigView();
}

function doDelete(ref, withKey) {
  if (!ref || view.deleteBusy) return;
  view.deleteBusy = true;
  const params = { action: 'delete_model', model_ref: ref, delete_api_key: !!withKey };
  if (view.deleteActiveAs) params.active_model = view.deleteActiveAs;
  view.deleteActiveAs = '';
  sendConfigSet(params);
  view.deleteConfirm = null;
  view.page = 'models';
  renderConfigView();
  setTimeout(() => { view.deleteBusy = false; }, 300);
}

function doSaveProvider() {
  const v = view.formValues;
  const err = form.validateProviderForm(v);
  if (err) { setFormError(err); return; }
  const m = view.formEditing ? modelByRef(view, view.formEditing) : null;
  const params = {
    action: 'upsert_model',
    model_ref: view.formEditing || undefined,
    api_key: v.apiKey || undefined,
    model: {
      provider: v.provider.trim(),
      model: v.model.trim(),
      base_url: v.endpoint.trim(),
      context_window: form.parsePositiveInt(v.contextWindow),
      max_output_tokens: form.parsePositiveInt(v.maxOutputTokens),
      strengths: form.splitCSV(v.strengths),
      subtask_for: form.splitCSV(v.subtaskFor),
      reasoning: m ? m.reasoning : null,
    },
  };
  sendConfigSet(params);
  view.formMode = null;
  renderConfigView();
}

// ============= Form 工厂 =============

const form = createConfigForm({
  getView: () => view,
  render: renderConfigView,
  setFormError,
  send,
  joinRef,
  modelByRef: (ref) => modelByRef(view, ref),
  escapeHtml,
  modal,
});

// pickProviderKind 弹出"选择 Provider 协议标准"模态。
function pickProviderKind() {
  return modal.pickFromList({
    title: '选择 Provider 协议标准',
    message: 'Suna 只暴露 openai 和 anthropic 两个标准协议。任何兼容中转都通过填写对应端点 + 协议 key 来表达。',
    options: [
      { value: 'openai', label: 'OpenAI', hint: 'OpenAI 标准 Responses / Chat Completions；端点形如 https://api.openai.com/v1' },
      { value: 'anthropic', label: 'Anthropic', hint: 'Anthropic 标准 /v1/messages；端点形如 https://api.anthropic.com' },
    ],
    emptyText: '没有可选协议',
    confirmText: '取消',
  });
}
