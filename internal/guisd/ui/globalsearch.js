/* globalsearch.js — 全局搜索面板（Ctrl+Shift+F 跨文件搜索内容）。
   调用 /api/fs/search API，按文件分组展示匹配结果。
   依赖 core.js 的 api/fileIcon/state/joinPath/delegate/escapeHtml，editor.js 的 openFile/gotoLine。 */

import { $, api, fileIcon, state, joinPath, delegate, escapeHtml } from './core.js';
import { openFile, gotoLine } from './editor.js';

let searchTimer = null;
let currentQuery = '';

// showSearch 打开搜索面板并聚焦输入框。
export function showSearch() {
  $('searchPanel').classList.add('show');
  $('searchInput').focus();
}

export function hideSearch() {
  $('searchPanel').classList.remove('show');
}

// doSearch 300ms 防抖触发实际搜索，避免每次按键都打后端。
function doSearch() {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(runSearch, 300);
}

async function runSearch() {
  const query = $('searchInput').value.trim();
  const resultsEl = $('searchResults');
  currentQuery = query;

  if (!query) {
    resultsEl.innerHTML = '';
    return;
  }

  const isRegex = $('searchRegex').checked;
  const ignoreCase = !$('searchCase').checked;

  resultsEl.innerHTML = '<div class="search-empty">搜索中...</div>';

  try {
    const params = new URLSearchParams({ query, isRegex: String(isRegex), ignoreCase: String(ignoreCase) });
    const data = await api('/api/fs/search?' + params);
    // 如果用户在请求期间改了输入，丢弃过时结果。
    if (currentQuery !== query) return;
    renderResults(data.results || [], query);
  } catch (err) {
    resultsEl.innerHTML = '<div class="search-empty">错误: ' + escapeHtml(err.message) + '</div>';
  }
}

function renderResults(results, query) {
  const resultsEl = $('searchResults');
  if (results.length === 0) {
    resultsEl.innerHTML = '<div class="search-empty">未找到结果</div>';
    return;
  }

  // 批插入：DocumentFragment 一次性 append，避免 N 次 reflow。
  const frag = document.createDocumentFragment();
  let totalHits = 0;
  for (const file of results) {
    totalHits += file.matches.length;
    const group = document.createElement('div');
    group.className = 'search-file-group';
    group.dataset.path = file.path;

    const name = file.path.split('/').pop();
    const header = document.createElement('div');
    header.className = 'search-file-header';
    header.innerHTML = fileIcon(name, false) + ' ' + escapeHtml(name)
      + ' <span class="search-file-count">' + file.matches.length + '</span>'
      + ' <span style="color:var(--text-faint);font-weight:400;font-size:10px">' + escapeHtml(file.path) + '</span>';
    group.append(header);

    for (const hit of file.matches) {
      const hitEl = document.createElement('div');
      hitEl.className = 'search-hit';
      hitEl.dataset.path = file.path;
      hitEl.dataset.line = String(hit.line);
      hitEl.innerHTML = '<span class="sh-line">' + hit.line + '</span>' + highlightPreview(hit.preview, query);
      group.append(hitEl);
    }
    frag.append(group);
  }

  // 顶部插入总数摘要。
  const summary = document.createElement('div');
  summary.className = 'search-file-header';
  summary.style.borderBottom = '1px solid var(--border)';
  summary.style.marginBottom = '4px';
  summary.textContent = '在 ' + results.length + ' 个文件中找到 ' + totalHits + ' 处匹配';
  frag.insertBefore(summary, frag.firstChild);

  resultsEl.innerHTML = '';
  resultsEl.append(frag);
}

// highlightPreview 在预览文本中高亮匹配项。
function highlightPreview(text, query) {
  const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return escaped.replace(new RegExp(escapedQuery, 'gi'), '<mark>$&</mark>');
}

// 搜索结果点击用事件代理：单一监听器处理所有结果项，
// 通过 data-path / data-line 在 click 时打开文件并跳转。
delegate($('searchResults'), 'click', '.search-hit', (e, el) => {
  const path = el.dataset.path;
  const line = Number(el.dataset.line);
  const name = path.split('/').pop();
  const fullPath = joinPath(state.workspaceRoot, path);
  openFile(fullPath, name);
  gotoLine(fullPath, line);
});

export function initSearch() {
  $('searchInput').addEventListener('input', doSearch);
  $('searchRegex').addEventListener('change', doSearch);
  $('searchCase').addEventListener('change', doSearch);
  $('searchCloseBtn').addEventListener('click', hideSearch);
}
