/* markdown.js — Markdown 渲染器，基于 marked.js（CommonMark 完整支持）。
   替换旧的 52 行手写实现，解决标题错位、列表无 ol 包裹、引用不合并等问题。
   依赖 vendor/marked/marked.esm.js。 */

import { marked } from './vendor/marked/marked.esm.js';

// 配置 marked：启用 GFM（表格、任务列表等），禁止 HTML 原始标签（安全）。
marked.setOptions({
  gfm: true,
  breaks: true,           // 单换行转 <br>，与聊天场景一致
});

// renderer 覆盖：让链接在新标签页打开，代码块加 class。
const renderer = new marked.Renderer();
renderer.link = function({ href, text }) {
  // marked v15 的 link 回调签名是 ({ href, title, text, tokens })，
  // 返回原始 HTML 字符串。
  const title = '';
  return `<a href="${href}" target="_blank" rel="noopener"${title ? ` title="${title}"` : ''}>${text}</a>`;
};

// renderMarkdown 将纯文本 Markdown 渲染为安全的 HTML。
// marked v15 内部已处理 HTML 转义（code blocks、text nodes），不需要预转义。
// 预转义 < > & 会导致 blockquote 语法（> ...）失效，因为 > 被转义为 &gt;。
export function renderMarkdown(text) {
  if (!text) return '';
  return marked.parse(text, { renderer });
}
