/* main.js — 入口模块，协调各模块初始化 + 全局快捷键 + 中央编辑区的视图切换。
   依赖 core.js 的 state/$/api/modal/detectPathSep，fs.js/editor.js/terminal.js/chat.js，
   quickopen.js/globalsearch.js/menu.js，以及特殊 tab 用的 config.js/help.js。 */

import { $, api, modal, state, detectPathSep } from './core.js';
import { getState, setState, subscribe } from './state.js';
import { initFileTree, refreshTree, goToWorkspaceRoot } from './fs.js';
import { initTerminal, initTermResizer, createNewTerminal, clearActiveTerminal } from './terminal.js';
import { connectChat, initChat, renderWelcomeIfEmpty, applyInputAvailability, sendAgentRequest, onConfigState } from './chat.js';
import { suspendTabs, resumeTabs, isTabsSuspended, closeCurrentTab, switchTabRelative } from './editor.js';
import { showQuickOpen, hideQuickOpen, initQuickOpen } from './quickopen.js';
import { showSearch, hideSearch, initSearch } from './globalsearch.js';
import { renderConfigView, updateState as updateConfigPanelState, bindAgentSend as bindConfigSend } from './config.js';
import { renderHelpView } from './help.js';
import { createMenuBar } from './menu.js';

// editorMode 记录中央编辑区当前显示的是文件视图（'file'）还是某个特殊 tab。
// 切到 special tab 之前调 editor.suspendTabs() 把文件视图摘下；
// 切回文件视图时调 editor.resumeTabs() 把文件视图重新挂回。
// 模型数据（textarea.value / scrollTop / selection）始终在 entry 内存中，零丢失。
let editorMode = 'file';

// setEditorMode 切换中央编辑区到指定模式。'file' / 'config' / 'help'。
// 不缓存 innerHTML 字符串，直接由 editor.js 控制 DOM 挂载/卸载。
function setEditorMode(mode) {
  const body = $('editorBody');
  const tabs = $('editorTabs');
  if (!body || !tabs) return;
  const specialTabs = tabs.querySelectorAll('.editor-tab.special');
  specialTabs.forEach(t => t.classList.remove('active'));
  if (mode === 'file') {
    // 从 special 退回 file：清空 editorBody（移除 special view），然后 resume 文件视图。
    if (isTabsSuspended()) {
      while (body.firstChild) body.removeChild(body.firstChild);
      resumeTabs();
    }
    editorMode = 'file';
    document.body.dataset.editorMode = 'file';
    tabs.querySelectorAll('.editor-tab:not(.special)').forEach(t => t.classList.remove('active'));
    return;
  }
  // 进入 special tab：先把文件视图挂起（如果还在），再清空 editorBody 渲染 special。
  if (editorMode === 'file' && !isTabsSuspended()) {
    suspendTabs();
  }
  while (body.firstChild) body.removeChild(body.firstChild);
  editorMode = mode;
  document.body.dataset.editorMode = mode;
  tabs.querySelectorAll('.editor-tab:not(.special)').forEach(t => t.classList.remove('active'));
  const target = tabs.querySelector('.editor-tab.special[data-special="' + mode + '"]');
  if (target) target.classList.add('active');
  if (mode === 'config') renderConfigView();
  else if (mode === 'help') renderHelpView();
  // 同步 hash，便于刷新恢复。
  history.replaceState(null, '', '#' + mode);
}

// 监听特殊 tab 的点击：data-special 决定目标模式。再点同一 tab 切回 file。
function bindSpecialTabs() {
  const tabs = $('editorTabs');
  if (!tabs) return;
  tabs.addEventListener('click', (e) => {
    const tab = e.target.closest('.editor-tab.special');
    if (!tab) return;
    const kind = tab.dataset.special;
    if (!kind) return;
    if (editorMode === kind) {
      setEditorMode('file');
    } else {
      setEditorMode(kind);
    }
  });
}

// 文件 tab 被点击时（editor.js 内部已处理 tab 切换和编辑器焦点），
// 我们也把 editorMode 切回 'file'，并清空 special tab 的 active 态。
function bindFileTabsReset() {
  const tabs = $('editorTabs');
  if (!tabs) return;
  tabs.addEventListener('click', (e) => {
    const tab = e.target.closest('.editor-tab:not(.special)');
    if (!tab) return;
    if (editorMode !== 'file') {
      editorMode = 'file';
      document.body.dataset.editorMode = 'file';
      tabs.querySelectorAll('.editor-tab.special').forEach(t => t.classList.remove('active'));
      history.replaceState(null, '', '#file');
    }
  });
}

// 从 URL hash 恢复初始模式。刷新或直接访问 #config / #help 时也生效。
function restoreModeFromHash() {
  const h = (location.hash || '').replace(/^#/, '');
  if (h === 'config' || h === 'help') setEditorMode(h);
}

// initMenuBar 挂载顶部菜单栏（VSCode 风格），并把现有功能挂到菜单项。
// 菜单结构在 menu.js 内部静态定义；此处只负责把动作注入。
// 注：菜单项 disabled 状态由 getContext() 动态计算。
function initMenuBar() {
  const menuBar = createMenuBar({
    actions: {
      'open-quickopen': showQuickOpen,
      'open-search': showSearch,
      'close-tab': closeCurrentTab,
      'go-home': goToWorkspaceRoot,
      'toggle-chat': toggleChatPanel,
      'open-config': () => setEditorMode('config'),
      'open-help': () => setEditorMode('help'),
      'new-term': createNewTerminal,
      'clear-term': clearActiveTerminal,
      'about': showAbout,
    },
    getContext: () => ({
      hasActiveFile: !!state.activeFile,
    }),
  });
  menuBar.mount($('menuBar'));
}

// chat.js config.state 通知的回调：刷新配置面板。
function onConfigStateChanged(snapshot) {
  updateConfigPanelState(snapshot);
}

function initConfigBridge() {
  bindConfigSend((method, params) => sendAgentRequest(method, params));
  onConfigState(onConfigStateChanged);
}

// init 获取状态、初始化文件树/终端/聊天。
async function init() {
  try {
    const status = await api('/api/status');
    state.workspaceRoot = status.cwd || '.';
    state.pathSep = detectPathSep(state.workspaceRoot);
    state.currentRoot = state.workspaceRoot;
    $('cwdDisplay').textContent = state.workspaceRoot.length > 40
      ? '...' + state.workspaceRoot.slice(-37)
      : state.workspaceRoot;
    state.daemonConnected = status.daemonConnected;
    // 同步到中央 store，避免 chat.js 等模块读 state.daemonConnected 和 getState('daemon.connected') 不一致。
    setState('daemon.connected', status.daemonConnected);
    if (state.daemonConnected) {
      $('daemonDot').className = 'status-dot online';
      $('daemonStatus').textContent = 'Daemon 在线';
      connectChat();
    } else {
      $('daemonStatus').textContent = 'Daemon 离线';
    }
  } catch {
    // guisd 未就绪
  }
  renderWelcomeIfEmpty();
  applyInputAvailability();
  await refreshTree();
  initTerminal();
}

// 绑定 header 按钮
$('toggleChat').addEventListener('click', () => {
  $('app').classList.toggle('no-chat');
});

// toggleChatPanel 切换右侧聊天面板可见性，供菜单"视图 → 切换聊天面板"复用。
function toggleChatPanel() {
  $('app').classList.toggle('no-chat');
}

// bindActivityBar 绑定 Activity Bar 的点击事件。
// 通过 data-activity 判断目标视图，切换侧边栏内容和编辑区视图。
// Activity Bar 的 active 态切换在 activity-btn 上完成，与 editorMode 保持一致。
function bindActivityBar() {
  const bar = $('activityBar');
  if (!bar) return;
  bar.addEventListener('click', (e) => {
    const btn = e.target.closest('.activity-btn[data-activity]');
    if (!btn) return;
    const target = btn.dataset.activity; // files / config / help
    // 切换编辑区视图
    setEditorMode(target === 'files' ? 'file' : target);
    // 更新 Activity Bar 的 active 态
    bar.querySelectorAll('.activity-btn[data-activity]').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    // 切换侧边栏内容（后续阶段会拆分 config/help 侧栏）
    updateSidebarForActivity(target);
  });
}

// updateSidebarForActivity 根据 Activity Bar 切换侧边栏内容。
// 当前只有文件树视图；配置和帮助视图在后续阶段实现。
function updateSidebarForActivity(activity) {
  const sidebar = $('sidebar');
  if (!sidebar) return;
  const filesView = $('sidebarFiles');
  // 隐藏所有侧边栏视图
  if (filesView) filesView.style.display = 'none';
  // 显示目标视图
  if (activity === 'files' && filesView) filesView.style.display = '';
  // config 和 help 的侧边栏视图将在后续阶段添加
}

// showAbout 菜单"帮助 → 关于 Suna"弹窗：展示版本/项目简介等静态信息。
// 后续可改为读取 package.json / 远端版本，这里先以本地硬编码为主。
function showAbout() {
  modal.alert({
    title: '关于 Suna',
    message: 'Suna 是一款本地终端 AI Agent。\n本界面由内置 GUI 服务（guisd）提供。\n更多功能请参考帮助页（?）。',
    confirmText: '好的',
  });
}

// 全局快捷键
document.addEventListener('keydown', (e) => {
  // 浮层（QuickOpen / Search / Modal）打开时，方向键 / Enter / Esc 由各自模块处理，
  // 这里只判断编辑区焦点场景的快捷键。
  if ((e.ctrlKey || e.metaKey) && e.key === 'p' && !e.shiftKey) {
    e.preventDefault();
    showQuickOpen();
    return;
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'f' || e.key === 'F')) {
    e.preventDefault();
    showSearch();
    return;
  }
  // Ctrl+W：关闭当前 tab（VSCode 同款）。仅在编辑区是 file 视图时生效。
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'w' || e.key === 'W')) {
    if (editorMode === 'file' && state.activeFile) {
      e.preventDefault();
      closeCurrentTab();
    }
    return;
  }
  // Ctrl+Tab / Ctrl+Shift+Tab：在打开的 tab 之间循环切换。
  // 不拦截 Ctrl+Shift+Tab 浏览器自身的"上一个标签"行为，统一关掉避免误触。
  if ((e.ctrlKey || e.metaKey) && e.key === 'Tab' && editorMode === 'file') {
    e.preventDefault();
    switchTabRelative(e.shiftKey ? -1 : 1);
    return;
  }
  if (e.key === 'Escape') {
    hideQuickOpen();
    hideSearch();
    if (editorMode !== 'file') setEditorMode('file');
    return;
  }
  if (e.key === '?' && !e.ctrlKey && !e.metaKey) {
    const tag = (document.activeElement && document.activeElement.tagName) || '';
    if (tag !== 'INPUT' && tag !== 'TEXTAREA') {
      e.preventDefault();
      setEditorMode('help');
    }
  }
});

// 监听 hashchange（用户改 location.hash），让 #config / #help 也能从外部触发。
window.addEventListener('hashchange', () => {
  const h = (location.hash || '').replace(/^#/, '');
  if (h === 'config' || h === 'help' || h === 'file') {
    if (h === 'file' && editorMode !== 'file') setEditorMode('file');
    else if (h !== 'file' && editorMode !== h) setEditorMode(h);
  }
});

initFileTree();
initChat();
initConfigBridge();
initQuickOpen();
initSearch();
initTermResizer();
bindActivityBar();
bindSpecialTabs();
bindFileTabsReset();
initMenuBar();
init();
restoreModeFromHash();
