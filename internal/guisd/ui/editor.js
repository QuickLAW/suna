/* editor.js — 编辑器模块，多标签页 + overlay 语法高亮 + 行号。
   高亮原理：textarea 文字透明，叠加 pre.highlight 层显示着色 HTML，两层同步滚动。
   依赖 core.js 的 api/fileIcon/toast/state/$，以及 highlight.js。 */

import { $, api, fileIcon, toast, state, modal } from './core.js';
import { highlight, langForFile } from './highlight.js';

const editorTabs = $('editorTabs');
const editorBody = $('editorBody');
const fileTree = $('fileTree');
// openTabs 记录所有打开过的 tab（path -> entry）。entry 是模型 + 视图的合并：
// 数据（value/scrollTop/selection）始终在 entry.textarea 里，DOM 节点可能因特殊 tab 切换被 detach。
// 切换到 ⚙ 设置 / 📖 帮助 时，main.js 调用 suspendTabs() 暂存视图；
// 切回文件视图时调用 resumeTabs() 还原。模型不重建，状态零丢失。
const openTabs = new Map(); // path -> entry
let suspended = false; // 是否处于 suspend 状态
// 暂存 suspend 时的 active tab 路径。统一由 state.activeFile 承担，避免双源真相。
let suspendedActive = null;

// ============= 通用工具 =============

// formatSize 把字节数格式化为 B / KB / MB，避免 footer 里直接显示一大串数字。
function formatSize(n) {
  if (!n || n < 0) return '0 B';
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  return (n / (1024 * 1024)).toFixed(1) + ' MB';
}

// setActiveTreeNode 把文件树里对应 path 的节点标为 active 并滚到可见区域。
// 用于：打开文件 / 切换 tab / 从特殊视图 resume 回来时同步编辑区与树的选中态。
// 节点在折叠子树里时 scrollIntoView 也会展开外层滚动条，行为符合 VSCode。
function setActiveTreeNode(path) {
  if (!fileTree || !path) {
    if (fileTree) fileTree.querySelectorAll('.tree-node.active').forEach(n => n.classList.remove('active'));
    return;
  }
  fileTree.querySelectorAll('.tree-node.active').forEach(n => n.classList.remove('active'));
  // CSS.escape 防止路径里的特殊字符（:, . 等）破坏属性选择器。
  const sel = '.tree-node[data-path="' + CSS.escape(path) + '"]';
  const node = fileTree.querySelector(sel);
  if (!node) return;
  node.classList.add('active');
  // nearest：节点已在视口时不滚动，VSCode 同款行为。
  node.scrollIntoView({ block: 'nearest' });
}

// openFile 打开（或切换到）指定文件。
export function openFile(path, name) {
  if (openTabs.has(path)) {
    switchTab(path);
    return;
  }
  api('/api/fs/read?path=' + encodeURIComponent(path)).then(data => {
    const tab = document.createElement('div');
    tab.className = 'editor-tab active';
    tab.dataset.path = path;
    const icon = document.createElement('span');
    icon.textContent = fileIcon(name, false);
    const label = document.createElement('span');
    label.textContent = name;
    const close = document.createElement('span');
    close.className = 'close-btn';
    close.textContent = '×';
    close.addEventListener('click', (e) => { e.stopPropagation(); closeTab(path); });
    tab.append(icon, label, close);
    tab.addEventListener('click', () => switchTab(path));

    editorTabs.querySelectorAll('.editor-tab').forEach(t => t.classList.remove('active'));
    editorTabs.append(tab);

    // 编辑器容器：行号槽 + overlay 高亮层 + textarea。
    // 行号槽使用 inner 绝对定位层 + transform 模拟滚动：避免 gutter 自身
    // overflow 滚动条与 textarea 不一致导致对齐抖动，1~6 行号被父容器裁切。
    const editorWrap = document.createElement('div');
    editorWrap.className = 'editor-wrap';

    const gutter = document.createElement('div');
    gutter.className = 'editor-gutter';

    const gutterInner = document.createElement('div');
    gutterInner.className = 'editor-gutter-inner';
    gutter.append(gutterInner);

    const pre = document.createElement('pre');
    pre.className = 'editor-highlight';
    pre.setAttribute('aria-hidden', 'true');

    const ta = document.createElement('textarea');
    ta.className = 'editor-textarea';
    ta.value = data.content;
    ta.spellcheck = false;
    ta.dataset.original = data.content;

    editorWrap.append(gutter, pre, ta);

    // 同步 gutter 行号与 textarea 滚动位置。
    // 使用 transform 配合 will-change 走 GPU 合成，大文件也不会卡。
    let scrollSyncPending = false;
    const syncGutterScroll = () => {
      scrollSyncPending = false;
      gutterInner.style.transform = 'translateY(' + (-ta.scrollTop) + 'px)';
    };
    const requestScrollSync = () => {
      if (scrollSyncPending) return;
      scrollSyncPending = true;
      requestAnimationFrame(syncGutterScroll);
    };

    const footer = document.createElement('div');
    footer.className = 'editor-footer';
    footer.innerHTML = '<div class="editor-footer-info"><span>' + name + '</span><span>' + formatSize(data.size) + '</span><span>' + data.modTime + '</span></div>';
    const saveBtn = document.createElement('button');
    saveBtn.className = 'save-btn';
    saveBtn.textContent = 'Save';
    saveBtn.addEventListener('click', () => saveFile(path));
    footer.append(saveBtn);
    const lang = langForFile(name);
    // entry 保存模型 + 视图引用：editorWrap/footer 用于 suspend/resume，textarea/pre/gutter 用于渲染。
    const entry = { name, editorWrap, textarea: ta, pre, gutter, gutterInner, saveBtn, tab, footer, lang, label, highlightTimer: null };
    openTabs.set(path, entry);

    // 初始渲染高亮 + 行号
    renderHighlight(entry);

    // 输入时更新高亮、行号、修改标记（防抖避免大文件卡顿）
    ta.addEventListener('input', () => {
      scheduleHighlight(entry);
      const modified = ta.value !== ta.dataset.original;
      saveBtn.classList.toggle('show', modified);
      label.style.fontStyle = modified ? 'italic' : '';
      label.textContent = modified ? '● ' + name : name;
    });

    // 滚动同步：textarea 滚动时，行号槽用 transform 跟随；高亮层保持原本的 scrollTop 同步。
    ta.addEventListener('scroll', () => {
      pre.scrollTop = ta.scrollTop;
      pre.scrollLeft = ta.scrollLeft;
      requestScrollSync();
    });
    // 初始触发一次：把 transform 同步到当前 scrollTop（通常为 0）。
    requestScrollSync();

    // Tab 键插入空格而非切换焦点
    ta.addEventListener('keydown', (e) => {
      if (e.key === 'Tab') {
        e.preventDefault();
        const s = ta.selectionStart;
        const e2 = ta.selectionEnd;
        ta.value = ta.value.substring(0, s) + '  ' + ta.value.substring(e2);
        ta.selectionStart = ta.selectionEnd = s + 2;
        scheduleHighlight(entry);
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        saveFile(path);
      }
    });

    editorBody.innerHTML = '';
    editorBody.append(editorWrap, footer);
    state.activeFile = path;
    setActiveTreeNode(path);
    ta.focus();
  }).catch(err => {
    toast('读取文件失败: ' + err.message, 'error');
  });
}

// scheduleHighlight 防抖渲染高亮（150ms 延迟，避免每次按键全量重渲染）。
function scheduleHighlight(entry) {
  if (entry.highlightTimer) clearTimeout(entry.highlightTimer);
  entry.highlightTimer = setTimeout(() => {
    renderHighlight(entry);
    entry.highlightTimer = null;
  }, 150);
}

// renderHighlight 更新高亮层 HTML 和行号槽。
function renderHighlight(entry) {
  const { textarea: ta, pre, gutterInner, lang } = entry;
  const code = ta.value;
  pre.innerHTML = highlight(code, lang);

  // 行号：写入 gutter 内部的 inner 层，由 transform 模拟滚动。
  // 行数等于按 \n 切分的段数；末尾是否带 \n 不影响行号计数（split 会产生一段空串）。
  const lines = code.split('\n').length;
  let nums = '';
  for (let i = 1; i <= lines; i++) {
    nums += i + '\n';
  }
  gutterInner.textContent = nums;
}

// gotoLine 跳转到指定文件的指定行（1-based）。
// 如果文件未打开则先打开，文件加载完成后跳转。
// 不用 setTimeout 是因为 openFile 内部是 async fetch，时长不可控；
// 改为在 openFile 之后再用 rAF 轮询直到 entry 出现，避免假死或时序竞争。
export function gotoLine(path, line) {
  const entry = openTabs.get(path);
  if (!entry) {
    const name = path.split(/[\\/]/).pop();
    openFile(path, name);
    const waitAndJump = () => {
      const e = openTabs.get(path);
      if (e) {
        scrollToLine(e, line);
      } else {
        requestAnimationFrame(waitAndJump);
      }
    };
    requestAnimationFrame(waitAndJump);
    return;
  }
  switchTab(path);
  scrollToLine(entry, line);
}

// scrollToLine 滚动 textarea 到指定行并高亮该行。
function scrollToLine(entry, line) {
  const { textarea: ta, pre, gutterInner } = entry;
  const lines = ta.value.split('\n');
  if (line < 1 || line > lines.length) return;

  // 计算行偏移（每行约 13px * 1.6 行高 ≈ 21px）
  const lineHeight = 21;
  const scrollPos = (line - 1) * lineHeight;
  ta.scrollTop = scrollPos;
  pre.scrollTop = scrollPos;
  // 行号槽用 transform 跟随；直接同步到当前 scrollTop。
  gutterInner.style.transform = 'translateY(' + (-scrollPos) + 'px)';

  // 将光标移到该行开头
  let pos = 0;
  for (let i = 0; i < line - 1; i++) {
    pos += lines[i].length + 1; // +1 for \n
  }
  ta.focus();
  ta.setSelectionRange(pos, pos);
}

function switchTab(path) {
  const t = openTabs.get(path);
  if (!t) return;
  editorTabs.querySelectorAll('.editor-tab').forEach(tab => tab.classList.remove('active'));
  t.tab.classList.add('active');
  const editorWrap = t.textarea.parentElement;
  editorBody.innerHTML = '';
  editorBody.append(editorWrap, t.footer);
  state.activeFile = path;
  setActiveTreeNode(path);
  t.textarea.focus();
}

// closeCurrentTab 关闭当前激活 tab；供 Ctrl+W 调用。
// 已是异步：未保存时弹 modal 询问，用户点取消则中止。
export async function closeCurrentTab() {
  if (state.activeFile) await closeTab(state.activeFile);
}

async function closeTab(path) {
  const t = openTabs.get(path);
  if (!t) return;
  if (t.textarea.value !== t.textarea.dataset.original) {
    const ok = await modal.confirm({
      title: '未保存的更改',
      message: '"' + t.name + '" 有未保存的更改，确认关闭？',
      confirmText: '关闭',
      danger: true,
    });
    if (!ok) return;
  }
  if (t.highlightTimer) clearTimeout(t.highlightTimer);
  openTabs.delete(path);
  t.tab.remove();
  if (openTabs.size === 0) {
    editorBody.innerHTML = '<div class="editor-empty"><div class="editor-empty-icon">📄</div><div class="editor-empty-text">选择文件查看或编辑</div></div>';
    state.activeFile = null;
    setActiveTreeNode(null);
  } else {
    const first = openTabs.keys().next().value;
    switchTab(first);
  }
}

// switchTabRelative 在打开的 tab 之间相对切换；供 Ctrl+Tab / Ctrl+Shift+Tab 调用。
// delta = +1 下一个 tab，-1 上一个 tab。按打开顺序在 entry 列表中循环。
export function switchTabRelative(delta) {
  if (openTabs.size === 0) return;
  const order = Array.from(openTabs.keys());
  const cur = state.activeFile;
  let i = cur ? order.indexOf(cur) : -1;
  i = (i + delta + order.length) % order.length;
  switchTab(order[i]);
}

async function saveFile(path) {
  const t = openTabs.get(path);
  if (!t) return;
  try {
    await api('/api/fs/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, content: t.textarea.value })
    });
    t.textarea.dataset.original = t.textarea.value;
    t.saveBtn.classList.add('saved');
    t.saveBtn.textContent = '已保存';
    t.label.textContent = t.name;
    t.label.style.fontStyle = '';
    toast('文件已保存', 'success');
    setTimeout(() => { t.saveBtn.classList.remove('show', 'saved'); t.saveBtn.textContent = 'Save'; }, 1500);
  } catch (err) {
    toast('保存失败: ' + err.message, 'error');
  }
}

// ============= 视图挂起 / 恢复 =============

// suspendTabs 把所有打开 tab 的 DOM 节点从 editorBody 上 detach，但保留 entry 全部数据。
// 切换到 ⚙ 设置 / 📖 帮助 时由 main.js 调用，避免 special view 覆盖文件视图 DOM。
// 注意：textarea 的 value / scrollTop / selectionStart 等属性在 detach 期间不变，
// 但 display:none 会让 scrollTop 失效；因此 resume 时需重新同步滚动位置。
export function suspendTabs() {
  if (suspended) return;
  suspendedActive = state.activeFile;
  suspended = true;
  // 把每个 entry 的可见节点（editorWrap / footer）从 editorBody 摘下，但不丢弃引用。
  for (const entry of openTabs.values()) {
    if (entry.editorWrap && entry.editorWrap.parentNode) entry.editorWrap.remove();
    if (entry.footer && entry.footer.parentNode) entry.footer.remove();
  }
}

// resumeTabs 把之前 suspend 的 tab 视图挂回 editorBody。
// 调用方需保证 editorBody 当前为空（不要被 special view 占据）。
export function resumeTabs() {
  if (!suspended) return;
  suspended = false;
  // 按 openTabs 顺序重新 append，最后一个成为 active 视图。
  let lastEntry = null;
  for (const entry of openTabs.values()) {
    if (entry.editorWrap && entry.footer) {
      editorBody.append(entry.editorWrap, entry.footer);
      lastEntry = entry;
    }
  }
  if (lastEntry) {
    // 重新触发一次高亮与行号同步：textarea 在 detach 期间 scrollTop 不会被重置，但 innerHTML 切换过
    // 容易丢一次 layout，强制 scheduleHighlight 让 view 跟上模型。
    scheduleHighlight(lastEntry);
    requestAnimationFrame(() => {
      if (lastEntry.textarea) {
        lastEntry.textarea.scrollTop = lastEntry.textarea.scrollTop;
        lastEntry.pre.scrollTop = lastEntry.textarea.scrollTop;
      }
    });
  }
  state.activeFile = suspendedActive;
  setActiveTreeNode(suspendedActive);
}

// isTabsSuspended 返回当前 editor 是否处于 suspend 状态。
// suspend 由 main.js 在切到 ⚙ 设置 / 📖 帮助 特殊视图前调用，
// 切回 file 视图时调 resumeTabs 还原。用来判断 isTabSpecial 状态与自动恢复。
export function isTabsSuspended() { return suspended; }

// closeFile 显式关闭一个 tab。
export function closeFile(path) {
  const entry = openTabs.get(path);
  if (!entry) return;
  if (entry.tab && entry.tab.parentNode) entry.tab.remove();
  if (entry.editorWrap && entry.editorWrap.parentNode) entry.editorWrap.remove();
  if (entry.footer && entry.footer.parentNode) entry.footer.remove();
  openTabs.delete(path);
  if (state.activeFile === path) {
    state.activeFile = null;
    // 切到剩下的最后一个 tab
    const next = openTabs.keys().next();
    if (!next.done) switchTab(next.value);
    else setActiveTreeNode(null);
  }
}
