/* core.js — 共享工具：DOM 查询、state、网络请求、路径处理、HTML 转义、事件代理、防抖、模态对话框。
   前端所有模块都依赖本文件；保持依赖纯净，禁止反向依赖业务模块。 */

// ============= DOM 工具 =============

// $ 简化的 getElementById 简写。
export const $ = id => document.getElementById(id);

// el 用属性对象快速创建 DOM 元素。
// 用法：el('div', { class: 'tree-row', 'data-path': '/a' }, [child1, child2])
// attrs 支持 class / text / html / dataset / on* 事件 / 任意 setAttribute。
export function el(tag, attrs, children) {
  const node = document.createElement(tag);
  if (attrs) {
    for (const k in attrs) {
      const v = attrs[k];
      if (v == null || v === false) continue;
      if (k === 'class') node.className = v;
      else if (k === 'text') node.textContent = v;
      else if (k === 'html') node.innerHTML = v;
      else if (k.startsWith('on') && typeof v === 'function') {
        node.addEventListener(k.slice(2).toLowerCase(), v);
      } else if (k === 'dataset' && typeof v === 'object') {
        for (const dk in v) node.dataset[dk] = v[dk];
      } else if (v === true) {
        node.setAttribute(k, '');
      } else {
        node.setAttribute(k, v);
      }
    }
  }
  if (children) {
    for (let i = 0; i < children.length; i++) {
      const c = children[i];
      if (c == null) continue;
      if (typeof c === 'string' || typeof c === 'number') {
        node.appendChild(document.createTextNode(String(c)));
      } else {
        node.appendChild(c);
      }
    }
  }
  return node;
}

// ============= HTML 转义 =============

// escapeHtml 通用 HTML 转义。处理 & < > " ' 五个字符。
// 用于把任意字符串安全插入 innerHTML；不要用于属性名 / 模板插值。
export function escapeHtml(s) {
  const div = document.createElement('div');
  div.textContent = s == null ? '' : String(s);
  return div.innerHTML;
}

// ============= 事件代理 =============

// delegate 在 root 上注册一个事件代理监听器，匹配 selector 的元素触发时调用 handler。
// 与每节点 addEventListener 相比，事件代理只用一个全局监听器，支持动态内容。
// eventName 例如 'click'、'change'。
export function delegate(root, eventName, selector, handler) {
  root.addEventListener(eventName, (e) => {
    const target = e.target.closest(selector);
    if (target && root.contains(target)) {
      handler(e, target);
    }
  });
}

// ============= 防抖 / 节流 / rAF 批 =============

// debounce 返回一个去抖函数：连续调用时只执行最后一次（trailing edge）。
// 用于搜索框输入、resize 事件等场景，避免频繁触发。
export function debounce(fn, ms) {
  let timer = null;
  return function (...args) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), ms);
  };
}

// throttle 返回一个节流函数：保证在窗口内至少执行一次。
// 用于 scroll / 拖拽等高频事件，降低主线程压力。
export function throttle(fn, ms) {
  let last = 0;
  let pending = null;
  return function (...args) {
    const now = Date.now();
    const remaining = ms - (now - last);
    if (remaining <= 0) {
      last = now;
      fn.apply(this, args);
    } else if (!pending) {
      pending = setTimeout(() => {
        last = Date.now();
        pending = null;
        fn.apply(this, args);
      }, remaining);
    }
  };
}

// rafBatch 把多次调用合并为下一个 requestAnimationFrame 一次执行。
// 用于 streaming chunk 等高频 DOM 更新场景，避免每次都触发 reflow。
export function rafBatch(fn) {
  let pending = false;
  return function (...args) {
    if (pending) return;
    pending = true;
    requestAnimationFrame(() => {
      pending = false;
      fn.apply(this, args);
    });
  };
}

// ============= 全局 state =============

// 全局共享状态，跨模块读写。
export const state = {
  daemonConnected: false,
  workspaceRoot: '',
  pathSep: '/',
  currentRoot: '',
  activeFile: null,
};

// ============= HTTP 工具 =============

// api 封装 fetch + JSON 解析 + 错误处理。
// 优先读后端写入的 {error} 字段；非 OK 时抛 Error。
export async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

// ============= 路径处理 =============

// detectPathSep 从 workspace 绝对路径推断 OS 分隔符。
export function detectPathSep(absPath) {
  if (!absPath) return '/';
  if (/^[A-Za-z]:/.test(absPath) || absPath.includes('\\')) return '\\';
  return '/';
}

// joinPath 用 workspace 检测到的分隔符拼接路径。
// list/search 返回的相对路径用正斜杠，需转为 OS 原生分隔符以匹配 tree 返回格式。
export function joinPath(...parts) {
  const sep = state.pathSep;
  return parts
    .filter(p => p && p.length > 0)
    .map(p => p.replace(/[/\\]+/g, sep))
    .join(sep)
    .replace(new RegExp('\\' + sep + '+', 'g'), sep);
}

// normalizePath 把任意路径转为 workspace 使用的原生分隔符格式。
export function normalizePath(path) {
  if (!path) return '';
  return path.replace(/[/\\]+/g, state.pathSep);
}

// ============= Toast =============

// toast 显示短暂提示。
let toastTimer = null;
export function toast(msg, type = '') {
  const t = $('toast');
  t.textContent = msg;
  t.className = 'toast show ' + type;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.className = 'toast'; toastTimer = null; }, 2000);
}

// ============= 文件图标 =============

// fileIcon 根据文件扩展名返回 emoji 图标。
export function fileIcon(name, isDir) {
  if (isDir) return '📁';
  const ext = name.split('.').pop().toLowerCase();
  const map = {
    go: '🐹', js: '📜', ts: '📜', html: '🌐', css: '🎨', json: '📋', md: '📝', txt: '📄',
    py: '🐍', rs: '🦀', java: '☕', c: '🔧', cpp: '🔧', h: '🔧', sh: '⚙️', yml: '⚙️',
    yaml: '⚙️', toml: '⚙️', sql: '🗄️', png: '🖼️', jpg: '🖼️', jpeg: '🖼️', gif: '🖼️', svg: '🖼️'
  };
  return map[ext] || '📄';
}

// ============= 模态对话框（替代浏览器原生 prompt/confirm）============

// modal 提供简单的模态对话框 API，替代 window.prompt/confirm，统一风格。
// 用法：await modal.prompt({ title, message, defaultValue, placeholder, confirmText })
//      await modal.confirm({ title, message, danger, confirmText })
//      await modal.pickFromList({ title, message, options: [{value,label,hint}] })
//      await modal.alert({ title, message, confirmText })
export const modal = {
  // prompt 异步返回一个字符串（用户输入），取消时返回 null。
  prompt({ title = '请输入', message = '', defaultValue = '', placeholder = '', confirmText = '确定' } = {}) {
    return new Promise((resolve) => {
      const dlg = createModalShell(title, message, confirmText, true, false);
      const input = dlg.querySelector('.modal-input');
      input.value = defaultValue;
      input.placeholder = placeholder;
      setTimeout(() => { input.focus(); input.select(); }, 30);
      bindModalEvents(dlg, {
        onConfirm: () => resolve(input.value),
        onCancel: () => resolve(null),
      });
    });
  },

  // confirm 异步返回 true / false。
  confirm({ title = '请确认', message = '', confirmText = '确定', danger = false } = {}) {
    return new Promise((resolve) => {
      const dlg = createModalShell(title, message, confirmText, false, danger);
      bindModalEvents(dlg, {
        onConfirm: () => resolve(true),
        onCancel: () => resolve(false),
      });
    });
  },

  // alert 异步弹出一个只含"确定"按钮的对话框；主要用于"关于 / 提示"等单向通知。
  // 始终显示一个主按钮（无取消按钮），按 Enter / Esc 都视为确认。
  alert({ title = '提示', message = '', confirmText = '知道了' } = {}) {
    return new Promise((resolve) => {
      const dlg = createModalShell(title, message, confirmText, false, false);
      bindModalEvents(dlg, {
        onConfirm: () => resolve(true),
        onCancel: () => resolve(true),
      });
    });
  },

  // pickFromList 单选列表模态：用户点列表项后立即 resolve(options[i].value) 并关闭；
  // 取消时返回 null。options 每项 { value, label, hint? }。
  // 用于"从供应商拉取模型列表"等需要把列表 ID 回填到表单的场景。
  pickFromList({ title = '请选择', message = '', options = [], emptyText = '无可选项', confirmText = '关闭' } = {}) {
    return new Promise((resolve) => {
      const dlg = createListModalShell(title, message, options, emptyText, confirmText);
      let settled = false;
      const settle = (v) => {
        if (settled) return;
        settled = true;
        resolve(v);
        closeListModal(dlg);
      };
      // 单击列表项：立即确认。
      dlg.querySelector('.modal-list').addEventListener('click', (e) => {
        const item = e.target.closest('.modal-list-item');
        if (item) settle(item.dataset.value);
      });
      // 主按钮（"关闭"）：取消。
      dlg.querySelector('.modal-confirm').addEventListener('click', () => settle(null));
      // 点击 overlay 空白：取消。
      dlg.addEventListener('click', (e) => { if (e.target === dlg) settle(null); });
      // Enter / Esc：取消（列表模态不需要 Enter 确认，避免误选第一项）。
      dlg.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === 'Escape') {
          e.preventDefault();
          settle(null);
        }
      });
    });
  },
};

function createModalShell(title, message, confirmText, withInput, danger) {
  const overlay = el('div', { class: 'modal-overlay' });
  const box = el('div', { class: 'modal-box' });
  const inputHTML = withInput ? '<input class="modal-input" type="text" />' : '';
  box.innerHTML = ''
    + '<div class="modal-title">' + escapeHtml(title) + '</div>'
    + (message ? '<div class="modal-message">' + escapeHtml(message) + '</div>' : '')
    + inputHTML
    + '<div class="modal-actions">'
    + '  <button class="chat-btn ghost modal-cancel">取消</button>'
    + '  <button class="chat-btn ' + (danger ? 'primary danger' : 'primary') + ' modal-confirm">' + escapeHtml(confirmText) + '</button>'
    + '</div>';
  overlay.append(box);
  document.body.append(overlay);
  requestAnimationFrame(() => overlay.classList.add('show'));
  return overlay;
}

function bindModalEvents(dlg, { onConfirm, onCancel }) {
  const close = () => { dlg.classList.remove('show'); setTimeout(() => dlg.remove(), 150); };
  dlg.addEventListener('click', (e) => { if (e.target === dlg) { onCancel(); close(); } });
  dlg.querySelector('.modal-cancel').addEventListener('click', () => { onCancel(); close(); });
  dlg.querySelector('.modal-confirm').addEventListener('click', () => { onConfirm(); close(); });
  const input = dlg.querySelector('.modal-input');
  if (input) {
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') { e.preventDefault(); onConfirm(); close(); }
      else if (e.key === 'Escape') { e.preventDefault(); onCancel(); close(); }
    });
  }
  dlg.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') { e.preventDefault(); onCancel(); close(); }
  });
}

// createListModalShell 构造单选列表模态的 DOM；options 每项 { value, label, hint? }。
// 真实的事件绑定（click、keydown、close）由 pickFromList 调用方在创建后挂到 dlg.__resolve / dlg.__close。
// 这样 createListModalShell 不依赖外部 promise，符合 core.js 的纯 DOM 风格。
function createListModalShell(title, message, options, emptyText, confirmText) {
  const overlay = el('div', { class: 'modal-overlay modal-list-overlay' });
  const box = el('div', { class: 'modal-box modal-list-box' });
  let listHTML = '';
  if (!options || options.length === 0) {
    listHTML = '<div class="modal-list-empty">' + escapeHtml(emptyText) + '</div>';
  } else {
    listHTML = options.map((o) => {
      const hint = o.hint ? '<div class="modal-list-item-hint">' + escapeHtml(o.hint) + '</div>' : '';
      return '<div class="modal-list-item" data-value="' + escapeAttr(o.value) + '">'
        + '<div class="modal-list-item-label">' + escapeHtml(o.label) + '</div>'
        + hint
        + '</div>';
    }).join('');
  }
  box.innerHTML = ''
    + '<div class="modal-title">' + escapeHtml(title) + '</div>'
    + (message ? '<div class="modal-message">' + escapeHtml(message) + '</div>' : '')
    + '<div class="modal-list">' + listHTML + '</div>'
    + '<div class="modal-actions">'
    + '  <button class="chat-btn primary modal-confirm">' + escapeHtml(confirmText) + '</button>'
    + '</div>';
  overlay.append(box);
  document.body.append(overlay);
  requestAnimationFrame(() => overlay.classList.add('show'));
  return overlay;
}

function closeListModal(dlg) {
  dlg.classList.remove('show');
  setTimeout(() => dlg.remove(), 150);
}

function escapeAttr(v) {
  return String(v ?? '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
