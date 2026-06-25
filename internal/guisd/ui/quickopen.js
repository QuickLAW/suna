/* quickopen.js — 快速打开面板（Ctrl+P 模糊搜索文件名）。
   从 /api/fs/list 拉取文件列表，本地做模糊匹配排序。 */

import { $, api, fileIcon, state, joinPath } from './core.js';
import { openFile } from './editor.js';

let fileList = [];      // 缓存的文件路径列表
let fileListLoaded = false;
let selectedIndex = 0;
let filtered = [];

// ensureList 确保文件列表已加载（惰性加载，首次打开时请求）。
async function ensureList() {
  if (fileListLoaded) return;
  try {
    const data = await api('/api/fs/list');
    fileList = data.files || [];
    fileListLoaded = true;
  } catch {
    // 加载失败静默处理
  }
}

// fuzzyMatch 模糊匹配：query 的每个字符按顺序出现在 path 中即匹配。
// 返回匹配得分（越小越好），-1 表示不匹配。
function fuzzyMatch(query, path) {
  const q = query.toLowerCase();
  const p = path.toLowerCase();
  let qi = 0, score = 0, lastMatch = -1;
  for (let pi = 0; pi < p.length && qi < q.length; pi++) {
    if (p[pi] === q[qi]) {
      // 连续匹配加分
      score += (lastMatch === pi - 1) ? 0 : 10;
      // 文件名部分匹配加分（取最后一个 / 后的位置）
      if (pi > 0 && p[pi - 1] === '/') score -= 5;
      lastMatch = pi;
      qi++;
    } else {
      score++;
    }
  }
  return qi === q.length ? score : -1;
}

// showQuickOpen 打开面板并拉取文件列表。
export async function showQuickOpen() {
  $('quickOpenOverlay').classList.add('show');
  const input = $('quickOpenInput');
  input.value = '';
  input.focus();
  await ensureList();
  filterAndRender('');
}

// hideQuickOpen 关闭面板。
export function hideQuickOpen() {
  $('quickOpenOverlay').classList.remove('show');
}

// filterAndRender 根据输入过滤并渲染结果列表。
function filterAndRender(query) {
  const list = $('quickOpenList');
  list.innerHTML = '';

  if (!query) {
    // 无输入时显示全部（最多 50 条）
    filtered = fileList.slice(0, 50).map(p => ({ path: p, score: 0 }));
  } else {
    // 模糊匹配 + 排序
    const matched = fileList
      .map(p => ({ path: p, score: fuzzyMatch(query, p) }))
      .filter(m => m.score >= 0)
      .sort((a, b) => a.score - b.score)
      .slice(0, 50);
    filtered = matched;
  }

  if (filtered.length === 0) {
    list.innerHTML = '<div class="search-empty">未找到文件</div>';
    return;
  }

  selectedIndex = 0;
  for (let i = 0; i < filtered.length; i++) {
    const item = document.createElement('div');
    item.className = 'quick-open-item' + (i === 0 ? ' selected' : '');
    const name = filtered[i].path.split('/').pop();
    const dir = filtered[i].path.includes('/') ? filtered[i].path.slice(0, filtered[i].path.lastIndexOf('/')) : '';
    item.innerHTML = '<span class="qo-icon">' + fileIcon(name, false) + '</span>' +
      '<span class="qo-path">' + name +
      (dir ? ' <span style="color:var(--text-faint);font-size:11px">' + dir + '</span>' : '') +
      '</span>';
    item.addEventListener('click', () => openFromList(i));
    list.append(item);
  }
}

// openFromList 打开选中文件并关闭面板。
function openFromList(index) {
  if (index < 0 || index >= filtered.length) return;
  const relPath = filtered[index].path;
  // 用 OS 原生分隔符拼接完整路径，确保与文件树返回格式一致
  const fullPath = joinPath(state.workspaceRoot, relPath);
  const name = relPath.split('/').pop();
  openFile(fullPath, name);
  hideQuickOpen();
}

// selectRelative 上下移动选中项。
function selectRelative(delta) {
  if (filtered.length === 0) return;
  selectedIndex = Math.max(0, Math.min(filtered.length - 1, selectedIndex + delta));
  const items = $('quickOpenList').querySelectorAll('.quick-open-item');
  items.forEach((el, i) => el.classList.toggle('selected', i === selectedIndex));
  // 确保选中项可见
  const sel = items[selectedIndex];
  if (sel) sel.scrollIntoView({ block: 'nearest' });
}

// initQuickOpen 绑定事件。
export function initQuickOpen() {
  const overlay = $('quickOpenOverlay');
  const input = $('quickOpenInput');

  // 点击遮罩关闭
  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) hideQuickOpen();
  });

  // 输入过滤
  input.addEventListener('input', () => filterAndRender(input.value));

  // 键盘导航
  input.addEventListener('keydown', (e) => {
    switch (e.key) {
      case 'Escape':
        e.preventDefault();
        hideQuickOpen();
        break;
      case 'ArrowDown':
        e.preventDefault();
        selectRelative(1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        selectRelative(-1);
        break;
      case 'Enter':
        e.preventDefault();
        openFromList(selectedIndex);
        break;
    }
  });
}
