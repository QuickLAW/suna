/* terminal.js — 终端模块，支持多标签页。
   xterm.js 与 addon-fit 走本地 vendor（见 vendor/xterm/），不依赖 CDN。
   每个标签独立连接 PTY WebSocket，可创建/切换/关闭。
   依赖 core.js 的 $。 */

import { $ } from './core.js';
import { Terminal, FitAddon } from './vendor/xterm/index.js';

const terminals = new Map(); // id -> {tab, body, term, ptyWS, fitAddon}
let activeTermId = 0;
let termCounter = 0;

// createTab 创建并返回一个终端 tab DOM，绑定切换/关闭事件。
// createTerminal 与 createTerminalFallback 共享此 UI 构造，避免重复 DOM 拼接。
function createTab(id, onSelect, onClose) {
  const tab = document.createElement('div');
  tab.className = 'terminal-tab active';
  const label = document.createElement('span');
  label.textContent = '终端 ' + id;
  const close = document.createElement('span');
  close.className = 'tt-close';
  close.textContent = '×';
  close.addEventListener('click', (e) => { e.stopPropagation(); onClose(); });
  tab.append(label, close);
  tab.addEventListener('click', onSelect);
  return { tab, label, close };
}

// activateTab 在 tabsEl / bodiesEl 中只激活指定 id。
function activateTab(id, tabsEl, bodiesEl) {
  tabsEl.querySelectorAll('.terminal-tab').forEach(t => t.classList.remove('active'));
  bodiesEl.querySelectorAll('.terminal-body').forEach(b => b.style.display = 'none');
  const e = terminals.get(id);
  if (e) {
    e.tab.classList.add('active');
    e.body.style.display = 'block';
  }
}

// stripAnsi 去掉控制字符便于纯文本降级终端显示。
function stripAnsi(text) {
  return text.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '');
}

// isControlMsg 检查 WebSocket 消息是否是控制类 JSON（mode/error），不是终端输出。
function isControlMsg(data) {
  if (typeof data !== 'string' || !data.startsWith('{')) return false;
  try {
    const ctrl = JSON.parse(data);
    return ctrl.mode || ctrl.error;
  } catch {
    return false;
  }
}

// connectPTY 打开 PTY WebSocket 并绑定到 entry，data/close/error 行为由 entry 自身处理。
function connectPTY(entry, onText) {
  const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(wsProto + '//' + location.host + '/api/pty');
  entry.ptyWS = ws;
  ws.onopen = () => {
    onText(null, 'open');
    if (entry.term) sendResize(entry);
  };
  ws.onmessage = (e) => {
    if (typeof e.data === 'string') {
      if (isControlMsg(e.data)) return;
      onText(e.data, 'data');
    } else if (e.data instanceof Blob) {
      e.data.text().then(t => onText(t, 'data'));
    }
  };
  ws.onclose = () => { onText(null, 'close'); };
  ws.onerror = () => { onText(null, 'error'); };
}

// sendResize 把当前 cols/rows 发给 PTY 服务端。
function sendResize(entry) {
  if (entry.ptyWS && entry.ptyWS.readyState === WebSocket.OPEN && entry.term && entry.fitAddon) {
    entry.ptyWS.send(JSON.stringify({ type: 'resize', cols: entry.term.cols, rows: entry.term.rows }));
  }
}

// closeTerminal 关闭一个终端，关闭后若全部关完则自动开新。
function closeTerminal(id) {
  const entry = terminals.get(id);
  if (!entry) return;
  if (entry.ptyWS) { try { entry.ptyWS.close(); } catch {} }
  if (entry.term) { try { entry.term.dispose(); } catch {} }
  entry.tab.remove();
  entry.body.remove();
  terminals.delete(id);
  if (activeTermId === id) {
    if (terminals.size > 0) {
      switchTerminal(terminals.keys().next().value);
    } else {
      activeTermId = 0;
      createTerminal();
    }
  }
}

// switchTerminal 切换到指定终端标签。
function switchTerminal(id) {
  const e = terminals.get(id);
  if (!e) return;
  activeTermId = id;
  $('terminalTabs').querySelectorAll('.terminal-tab').forEach(t => t.classList.remove('active'));
  $('terminalBodies').querySelectorAll('.terminal-body').forEach(b => b.style.display = 'none');
  e.tab.classList.add('active');
  e.body.style.display = 'block';
  if (e.fitAddon) e.fitAddon.fit();
  if (e.term) e.term.focus();
}

// onWindowResize 全局窗口尺寸变化统一在这里 re-fit，避免每个终端各注册一份。
let resizeHandlerAttached = false;
function onWindowResize() {
  const e = terminals.get(activeTermId);
  if (e && e.fitAddon) e.fitAddon.fit();
}

// ============= 入口 =============

// initTermResizer 绑定 #termResizer 的拖拽：纵向调整 #terminalPanel 的高度。
// 走 body 的 pointermove 事件而不是 element 自身的，因为拖拽时鼠标会移出 resizer。
// 上下限由 CSS 的 .terminal-panel min/max 决定，JS 只在合法范围内同步高度。
// 终止时重新 fit 当前激活终端，避免 resize 消息晚到导致 xterm 显示错位。
export function initTermResizer() {
  const resizer = $('termResizer');
  const panel = $('terminalPanel');
  if (!resizer || !panel) return;
  const min = 80;        // 终端最小高度，避免拖成 0
  const max = window.innerHeight - 100; // 留出编辑区 + header
  let startY = 0;
  let startH = 0;
  let dragging = false;

  const onMove = (e) => {
    if (!dragging) return;
    const delta = startY - e.clientY; // 向上拖是正值 → 终端变大
    let h = startH + delta;
    if (h < min) h = min;
    if (h > max) h = max;
    panel.style.height = h + 'px';
  };
  const onUp = () => {
    if (!dragging) return;
    dragging = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    document.removeEventListener('pointermove', onMove);
    document.removeEventListener('pointerup', onUp);
    const e = terminals.get(activeTermId);
    if (e && e.fitAddon) e.fitAddon.fit();
  };
  resizer.addEventListener('pointerdown', (e) => {
    dragging = true;
    startY = e.clientY;
    startH = panel.offsetHeight;
    document.body.style.cursor = 'ns-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  });
}

// initTerminal 在 app 启动时调用：绑定新建按钮 + 立即建第一个终端。
export function initTerminal() {
  $('newTermBtn').addEventListener('click', () => createNewTerminal());
  $('clearTerm').addEventListener('click', () => clearActiveTerminal());
  if (!resizeHandlerAttached) {
    window.addEventListener('resize', onWindowResize);
    resizeHandlerAttached = true;
  }
  createNewTerminal();
}

// createNewTerminal 暴露给菜单"终端 → 新建终端"用。
// 内部就是原来的 createTerminal 逻辑（已重命名）。
export function createNewTerminal() {
  return createTerminal();
}

// clearActiveTerminal 暴露给菜单"终端 → 清空当前终端"用。
// 无活动终端时静默忽略。
export function clearActiveTerminal() {
  const e = terminals.get(activeTermId);
  if (!e) return;
  if (e.term) e.term.clear();
  if (e.fb) e.fb.textContent = '';
}

// createTerminal 用 xterm.js + addon-fit 创建新终端。
function createTerminal() {
  const id = ++termCounter;
  const tabsEl = $('terminalTabs');
  const bodiesEl = $('terminalBodies');
  const { tab } = createTab(id, () => switchTerminal(id), () => closeTerminal(id));
  tabsEl.append(tab);

  const body = document.createElement('div');
  body.className = 'terminal-body';
  body.style.display = 'block';
  bodiesEl.append(body);

  // xterm.js 不支持 CSS 变量，需要在创建时从 computed style 读取当前主题的十六进制值。
  // 这样未来支持浅色模式时只需改 CSS 变量，终端主题自动跟随。
  const cs = getComputedStyle(document.documentElement);
  const varOr = (name, fallback) => {
    const v = cs.getPropertyValue(name).trim();
    return v || fallback;
  };
  const term = new Terminal({
    fontFamily: 'var(--mono)',
    fontSize: 13,
    theme: {
      background: varOr('--bg-2', '#161b22'),
      foreground: varOr('--text', '#e6edf3'),
      cursor: varOr('--accent', '#6366f1'),
      selectionBackground: 'rgba(99,102,241,.3)',
      black: varOr('--border-light', '#484f58'),
      red: varOr('--red', '#f85149'),
      green: varOr('--green', '#3fb950'),
      yellow: varOr('--yellow', '#d29922'),
      blue: varOr('--blue', '#58a6ff'),
      magenta: varOr('--purple', '#bc8cff'),
      cyan: '#39c5cf',
      white: varOr('--text', '#e6edf3'),
    },
    cursorBlink: true,
    scrollback: 5000,
    allowProposedApi: true,
  });
  const fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.open(body);
  fitAddon.fit();
  term.writeln('\x1b[1;34m╭─────────────────────────────────────╮\x1b[0m');
  term.writeln('\x1b[1;34m│         Suna 终端 ' + id + '               │\x1b[0m');
  term.writeln('\x1b[1;34m╰─────────────────────────────────────╯\x1b[0m');
  term.writeln('');

  const entry = { id, tab, body, term, fitAddon, ptyWS: null };
  terminals.set(id, entry);
  activateTab(id, tabsEl, bodiesEl);
  activeTermId = id;

  // 终端输入 → PTY。
  term.onData(data => {
    if (entry.ptyWS && entry.ptyWS.readyState === WebSocket.OPEN) entry.ptyWS.send(data);
  });
  term.onResize(() => sendResize(entry));

  // PTY 输出 / 状态 → 终端。
  connectPTY(entry, (text, ev) => {
    if (ev === 'open') { term.writeln('\x1b[2m[终端已连接]\x1b[0m'); return; }
    if (ev === 'close') { term.writeln('\r\n\x1b[2m[终端已断开]\x1b[0m'); return; }
    if (ev === 'error') { term.writeln('\r\n\x1b[31m[终端错误]\x1b[0m'); return; }
    if (text) term.write(text);
  });
}
