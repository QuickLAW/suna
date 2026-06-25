/* fs.js — 文件树模块，含右键菜单（新建/删除/重命名）+ 树状态保留。
   依赖 core.js 的 api/fileIcon/state/$/toast/modal/delegate/el/debounce，以及 editor.js 的 openFile。 */

import { $, api, fileIcon, state, toast, joinPath, modal, delegate, el } from './core.js';
import { openFile } from './editor.js';

const tree = $('fileTree');

// expandedPaths 记录已展开的目录路径，刷新树时复用，保留用户的展开状态。
// 持久化到 sessionStorage，刷新页面也能恢复。
const SESSION_KEY = 'suna-tree-expanded';
let expandedPaths = new Set();
try { expandedPaths = new Set(JSON.parse(sessionStorage.getItem(SESSION_KEY) || '[]')); } catch {}
function persistExpanded() {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify([...expandedPaths]));
}

// saveCache 保存单个目录的 entries 到 _dirCache，用于在 DOM 重建时复用而不重新 IO。
const dirCache = new Map(); // path -> { ts, entries }

function isExpanded(path) { return expandedPaths.has(path); }
function setExpanded(path, on) {
  if (on) expandedPaths.add(path);
  else expandedPaths.delete(path);
  persistExpanded();
}

// buildNode 构造一个树节点 DOM。返回值：{ node, childContainer, loaded }
// loaded 用于延迟加载子目录（首次展开时再 IO）。
function buildNode(entry, depth) {
  const padding = 12 + depth * 14;
  const node = el('div', { class: 'tree-node', dataset: { path: entry.path, name: entry.name, isDir: String(entry.isDir) } });
  node.style.paddingLeft = padding + 'px';
  if (entry.isDir) {
    const chevron = el('span', { class: 'tree-chevron', text: '▶' });
    const icon = el('span', { class: 'tree-icon', text: '📁' });
    const label = el('span', { class: 'tree-label', text: entry.name });
    node.append(chevron, icon, label);

    const childContainer = el('div', { class: 'tree-children' });
    if (isExpanded(entry.path)) {
      chevron.classList.add('open');
      icon.textContent = '📂';
      // 标记为已展开但内容待加载（schedule expand 时再 IO）。
      childContainer._needsLoad = true;
    } else {
      childContainer.style.display = 'none';
    }
    return { node, childContainer, loaded: false };
  } else {
    const spacer = el('span', { class: 'tree-chevron', text: '' });
    const icon = el('span', { class: 'tree-icon', text: fileIcon(entry.name, false) });
    const label = el('span', { class: 'tree-label', text: entry.name });
    node.append(spacer, icon, label);
    if (entry.path === state.activeFile) node.classList.add('active');
    return { node, childContainer: null, loaded: true };
  }
}

// loadChildren 异步加载子目录内容到 childContainer，使用 DocumentFragment 批插入。
// 复用 expandedPaths 状态保持展开位置。
async function loadChildren(dirPath, childContainer, depth) {
  let entries;
  const cached = dirCache.get(dirPath);
  if (cached && Date.now() - cached.ts < 5000) {
    entries = cached.entries;
  } else {
    try {
      entries = await api('/api/fs/tree?path=' + encodeURIComponent(dirPath));
      dirCache.set(dirPath, { ts: Date.now(), entries });
    } catch (err) {
      childContainer.innerHTML = '<div class="tree-error">' + err.message + '</div>';
      return;
    }
  }
  const frag = document.createDocumentFragment();
  for (const e of entries) frag.append(buildNode(e, depth).node);
  if (entries.length === 0) frag.append(el('div', { class: 'tree-empty', text: '(empty)' }));
  childContainer.innerHTML = '';
  childContainer.append(frag);
  // 递归展开：子目录如果之前已展开，IO 加载后立即展开。
  for (const e of entries) {
    if (e.isDir && isExpanded(e.path)) {
      const childNode = childContainer.querySelector('.tree-node[data-path="' + cssEscape(e.path) + '"]');
      if (childNode) {
        const cc = childNode.nextElementSibling;
        if (cc && cc.classList.contains('tree-children') && !cc._loaded) {
          cc._loaded = true;
          cc.style.display = 'block';
          await loadChildren(e.path, cc, depth + 1);
        }
      }
    }
  }
}

// cssEscape 用一个等价的轻量转义，避免在 querySelector 里写 [data-path="..."] 因含特殊字符失败。
function cssEscape(s) {
  return s.replace(/[\\"]/g, '\\$&');
}

// loadDir 加载根目录到 container，保留展开状态。
async function loadDir(dirPath, container, depth) {
  let entries;
  try {
    entries = await api('/api/fs/tree?path=' + encodeURIComponent(dirPath));
  } catch (err) {
    container.innerHTML = '<div class="tree-error">' + err.message + '</div>';
    return;
  }
  const frag = document.createDocumentFragment();
  for (const e of entries) {
    const { node } = buildNode(e, depth);
    frag.append(node);
    if (e.isDir) {
      const cc = el('div', { class: 'tree-children' });
      if (isExpanded(e.path)) {
        cc._loaded = true;
        cc.style.display = 'block';
        node.querySelector('.tree-chevron').classList.add('open');
        node.querySelector('.tree-icon').textContent = '📂';
      } else {
        cc.style.display = 'none';
      }
      frag.append(cc);
      // 记录 childContainer 引用供后续展开事件使用
      node._childContainer = cc;
    }
  }
  container.innerHTML = '';
  container.append(frag);
  // 一次性递归展开所有 expandedPaths
  for (const e of entries) {
    if (e.isDir && isExpanded(e.path)) {
      const childNode = container.querySelector('.tree-node[data-path="' + cssEscape(e.path) + '"]');
      if (childNode && childNode._childContainer) {
        await loadChildren(e.path, childNode._childContainer, depth + 1);
      }
    }
  }
}

// refreshTree 重新加载根目录；保留展开状态。
export async function refreshTree() {
  await loadDir(state.currentRoot, tree, 0);
}

// 树节点点击（事件代理）：目录折叠 / 展开；文件打开。
// 单一 listener 处理整个 tree，节点动态增删不影响。
delegate(tree, 'click', '.tree-node', async (e, node) => {
  e.stopPropagation();
  const isDir = node.dataset.isDir === 'true';
  if (isDir) {
    const chevron = node.querySelector('.tree-chevron');
    const icon = node.querySelector('.tree-icon');
    const cc = node._childContainer || node.nextElementSibling;
    if (!cc || !cc.classList.contains('tree-children')) return;
    const opening = !chevron.classList.contains('open');
    chevron.classList.toggle('open', opening);
    icon.textContent = opening ? '📂' : '📁';
    cc.style.display = opening ? 'block' : 'none';
    setExpanded(node.dataset.path, opening);
    if (opening && !cc._loaded) {
      cc._loaded = true;
      await loadChildren(node.dataset.path, cc, countDepth(node) + 1);
    }
  } else {
    openFile(node.dataset.path, node.dataset.name);
  }
});

// countDepth 通过 paddingLeft 反推层级。
function countDepth(node) {
  const pl = parseInt(node.style.paddingLeft || '12', 10);
  return Math.max(0, Math.floor((pl - 12) / 14));
}

// ===== 右键菜单 =====

function showContextMenu(x, y, targetNode) {
  const menu = $('contextMenu');
  const isDir = targetNode ? targetNode.dataset.isDir === 'true' : false;
  const nodePath = targetNode ? targetNode.dataset.path : state.currentRoot;
  const nodeName = targetNode ? targetNode.dataset.name : '';

  const items = [];
  items.push({ label: '📄 新建文件', action: () => promptCreate(nodePath, false) });
  items.push({ label: '📁 新建文件夹', action: () => promptCreate(nodePath, true) });
  if (targetNode) {
    items.push({ sep: true });
    items.push({ label: '✏️ 重命名', action: () => promptRename(nodePath, nodeName) });
    items.push({ sep: true });
    items.push({ label: '🗑 删除', danger: true, action: () => doDelete(nodePath, nodeName) });
  }

  // 渲染（事件代理在外层监听 click；这里只放 markup）
  menu.innerHTML = '';
  const frag = document.createDocumentFragment();
  for (const item of items) {
    if (item.sep) { frag.append(el('div', { class: 'context-sep' })); continue; }
    frag.append(el('div', {
      class: 'context-item' + (item.danger ? ' danger' : ''),
      text: item.label,
      dataset: { act: 'ctx' },
    }));
  }
  menu.append(frag);
  // 记录当前菜单的 actions 数组，用 index 关联。
  menu._actions = items.filter(it => !it.sep).map(it => it.action);
  menu.style.left = Math.min(x, window.innerWidth - 180) + 'px';
  menu.style.top = Math.min(y, window.innerHeight - 200) + 'px';
  menu.classList.add('show');
}

function hideContextMenu() {
  $('contextMenu').classList.remove('show');
}

// promptCreate 用 modal 输入框创建文件/文件夹。
async function promptCreate(parentPath, isDir) {
  const name = await modal.prompt({
    title: isDir ? '新建文件夹' : '新建文件',
    placeholder: isDir ? '新文件夹名' : '新文件名（含后缀）',
    confirmText: '创建',
  });
  if (!name || !name.trim()) return;
  const newPath = joinPath(parentPath, name.trim());
  try {
    await api('/api/fs/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: newPath, isDir }),
    });
    toast((isDir ? '文件夹' : '文件') + '已创建', 'success');
    dirCache.delete(parentPath);
    await refreshTree();
  } catch (err) {
    toast('创建失败: ' + err.message, 'error');
  }
}

// promptRename 用 modal 输入框重命名。
async function promptRename(oldPath, oldName) {
  const newName = await modal.prompt({
    title: '重命名',
    defaultValue: oldName,
    confirmText: '重命名',
  });
  if (!newName || !newName.trim() || newName === oldName) return;
  const sep = state.pathSep || '/';
  const idx = oldPath.lastIndexOf(sep);
  const dir = idx > 0 ? oldPath.slice(0, idx) : '';
  const newPath = joinPath(dir, newName.trim());
  try {
    await api('/api/fs/rename', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ oldPath, newPath }),
    });
    toast('已重命名', 'success');
    dirCache.delete(dir);
    setExpanded(oldPath, false);
    await refreshTree();
  } catch (err) {
    toast('重命名失败: ' + err.message, 'error');
  }
}

// doDelete 模态确认后删除。
async function doDelete(path, name) {
  const ok = await modal.confirm({
    title: '确认删除',
    message: '确认删除 "' + name + '"？此操作不可撤销。',
    confirmText: '删除',
    danger: true,
  });
  if (!ok) return;
  try {
    await api('/api/fs/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    });
    toast('已删除', 'success');
    dirCache.clear();
    setExpanded(path, false);
    await refreshTree();
  } catch (err) {
    toast('删除失败: ' + err.message, 'error');
  }
}

// initFileTree 绑定文件树和右键菜单事件。
export function initFileTree() {
  $('goHome').addEventListener('click', () => goToWorkspaceRoot());

  // 树区域右键：节点右键打开菜单；空白处右键在根目录操作。
  tree.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    const node = e.target.closest('.tree-node');
    showContextMenu(e.clientX, e.clientY, node);
  });

  // 右键菜单项点击（事件代理）。
  delegate($('contextMenu'), 'click', '.context-item', (e, item) => {
    const items = [...$('contextMenu').querySelectorAll('.context-item')];
    const idx = items.indexOf(item);
    const action = $('contextMenu')._actions[idx];
    hideContextMenu();
    if (action) action();
  });

  // 全局关闭：点击 / 右键非菜单 / 非树区域。
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.context-menu')) hideContextMenu();
  });
  document.addEventListener('contextmenu', (e) => {
    if (!e.target.closest('.tree') && !e.target.closest('.context-menu')) hideContextMenu();
  });
}

// goToWorkspaceRoot 暴露给菜单"文件 → 回到工作目录"用。
// 与 #goHome 按钮的逻辑一致：把当前根目录设回 workspaceRoot 并刷新树。
export async function goToWorkspaceRoot() {
  state.currentRoot = state.workspaceRoot || '.';
  $('cwdDisplay').textContent = state.workspaceRoot || '.';
  await refreshTree();
}
