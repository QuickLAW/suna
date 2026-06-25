/* chat-stream.js — Agent 流式消息处理：chunk 累积、思考折叠、工具调用卡片、Usage 展示。
   内部以工厂模式接收 chat.js 注入的依赖，避免共享全局变量。
   依赖 core.js 的 escapeHtml / rafBatch / debounce，markdown.js 的 renderMarkdown。 */

import { escapeHtml, rafBatch } from './core.js';
import { renderMarkdown } from './markdown.js';

// createChatStream 构造流式渲染器。
// 依赖：
//   - chatMessages / scrollIfAtBottom: DOM 与自动滚动
//   - agentSend: 发送 JSON-RPC 请求
//   - addChatMsg(role, content): 添加新消息节点
//   - currentAssistantEl / reasoningStartTime / toolCards: 共享状态
export function createChatStream(deps) {
  const { chatMessages, scrollIfAtBottom, addChatMsg, state } = deps;

  // ===== 流式 chunk 渲染 =====
  // 浏览器对单次 textContent += chunk 会强制 reflow；高频流式时（agent.stream 推送很快）卡顿明显。
  // 用 rafBatch 把一个动画帧内的多个 chunk 合并为一次 textContent 写入，把 N 次 reflow 压成 1 次。
  const pendingChunks = []; // [{ chunk, isError }]
  const flushChunks = () => {
    if (pendingChunks.length === 0) return;
    let target = state.currentAssistantEl;
    if (!target) {
      target = addChatMsg('assistant', '');
      state.currentAssistantEl = target;
    }
    const content = target.querySelector('.chat-msg-content');
    // 把所有 chunk 拼接后一次写入；如果首 chunk 是 error，标记红色。
    const text = pendingChunks.map(c => c.chunk).join('');
    content.textContent += text;
    if (pendingChunks.some(c => c.isError)) content.style.color = 'var(--red)';
    pendingChunks.length = 0;
    scrollIfAtBottom();
  };
  const scheduleFlush = rafBatch(flushChunks);

  function appendStreamChunk(chunk, isError) {
    pendingChunks.push({ chunk, isError });
    scheduleFlush();
  }

  function finalizeAssistantMessage() {
    // 末尾也走一次 flush，确保最后一帧 chunk 不丢。
    flushChunks();
    const target = state.currentAssistantEl;
    if (!target) return;
    const content = target.querySelector('.chat-msg-content');
    if (!content) return;
    const text = content.textContent;
    if (text.trim()) {
      content.innerHTML = renderMarkdown(text);
      content.classList.add('markdown');
    }
    scrollIfAtBottom();
  }

  // ===== 思考过程折叠 =====

  function appendReasoning(chunk) {
    let target = state.currentAssistantEl;
    if (!target) {
      target = addChatMsg('assistant', '');
      state.currentAssistantEl = target;
    }
    if (!state.reasoningStartTime) state.reasoningStartTime = Date.now();

    let toggle = target.querySelector('.chat-reasoning-toggle');
    let body = target.querySelector('.chat-reasoning-body');

    if (!toggle) {
      toggle = document.createElement('div');
      toggle.className = 'chat-reasoning-toggle';
      toggle.innerHTML = '<span class="rt-icon">▶</span><span class="rt-label">思考过程</span><span class="rt-duration"></span>';
      toggle.addEventListener('click', () => {
        toggle.classList.toggle('open');
        body.classList.toggle('show');
        toggle.querySelector('.rt-icon').textContent = toggle.classList.contains('open') ? '▼' : '▶';
      });
      const contentEl = target.querySelector('.chat-msg-content');
      target.insertBefore(toggle, contentEl);
    }
    if (!body) {
      body = document.createElement('div');
      body.className = 'chat-reasoning-body';
      const contentEl = target.querySelector('.chat-msg-content');
      target.insertBefore(body, contentEl);
    }

    body.textContent += chunk;
    const duration = Math.round((Date.now() - state.reasoningStartTime) / 100) / 10;
    toggle.querySelector('.rt-duration').textContent = duration > 0 ? duration + 's' : '';
    scrollIfAtBottom();
  }

  // ===== 工具调用卡片 =====

  function createToolCard(p) {
    const card = document.createElement('div');
    card.className = 'tool-card tool-running';
    const header = document.createElement('div');
    header.className = 'tool-card-header';
    const toolName = p.tool || '未知工具';
    const intent = p.intent || '';
    header.innerHTML = '<span class="tc-icon">▶</span>' +
      '<span class="tc-tool">🔧 ' + escapeHtml(toolName) + '</span>' +
      (intent ? '<span class="tc-intent">' + escapeHtml(intent) + '</span>' : '<span class="tc-intent"></span>') +
      '<span class="tc-spinner"></span>' +
      '<span class="tc-status running">执行中</span>';

    const body = document.createElement('div');
    body.className = 'tool-card-body';
    if (p.params && Object.keys(p.params).length > 0) {
      const paramsEl = document.createElement('div');
      paramsEl.className = 'tool-card-params';
      paramsEl.innerHTML = '<span class="tcp-label">参数:</span>\n' + escapeHtml(JSON.stringify(p.params, null, 2));
      body.append(paramsEl);
    }
    const resultEl = document.createElement('div');
    resultEl.className = 'tool-card-result';
    resultEl.style.display = 'none';
    body.append(resultEl);

    card.append(header, body);
    header.addEventListener('click', () => {
      header.classList.toggle('open');
      body.classList.toggle('show');
      header.querySelector('.tc-icon').textContent = header.classList.contains('open') ? '▼' : '▶';
    });

    const empty = document.getElementById('chatEmpty');
    if (empty) empty.style.display = 'none';
    chatMessages.append(card);
    scrollIfAtBottom();

    if (p.id) state.toolCards.set(p.id, { card, header, body, resultEl });
  }

  function updateToolCard(p) {
    let entry;
    if (p.id && state.toolCards.has(p.id)) {
      entry = state.toolCards.get(p.id);
      state.toolCards.delete(p.id);
    } else {
      // 没有 ID 匹配：找最后一个 running 的卡片兜底。
      const cards = chatMessages.querySelectorAll('.tool-card');
      for (let i = cards.length - 1; i >= 0; i--) {
        const h = cards[i].querySelector('.tool-card-header');
        if (h.querySelector('.tc-spinner')) {
          entry = {
            card: cards[i],
            header: h,
            body: cards[i].querySelector('.tool-card-body'),
            resultEl: cards[i].querySelector('.tool-card-result'),
          };
          break;
        }
      }
    }
    if (!entry) return;

    const spinner = entry.header.querySelector('.tc-spinner');
    if (spinner) spinner.remove();
    entry.card.classList.remove('tool-running');

    const status = entry.header.querySelector('.tc-status');
    if (p.error) {
      entry.card.classList.add('tool-error');
      status.className = 'tc-status error';
      status.textContent = '失败';
    } else {
      entry.card.classList.add('tool-success');
      status.className = 'tc-status success';
      status.textContent = '完成';
    }
    if (p.result) {
      entry.resultEl.textContent = p.result;
      if (p.error) entry.resultEl.classList.add('error');
      entry.resultEl.style.display = 'block';
    }
    if (p.result_truncated) {
      const trunc = document.createElement('div');
      trunc.className = 'tool-card-truncated';
      trunc.textContent = '结果已截断（原始 ' + (p.result_bytes || '?') + ' 字节）';
      entry.body.append(trunc);
    }
    scrollIfAtBottom();
  }

  // ===== Usage 信息条 =====

  function showUsageBar(p) {
    if (!state.currentAssistantEl) return;
    const oldBar = state.currentAssistantEl.querySelector('.usage-bar');
    if (oldBar) oldBar.remove();

    const bar = document.createElement('div');
    bar.className = 'usage-bar';
    const items = [];
    if (p.input_tokens) items.push('<span class="ub-item"><span class="ub-label">输入</span><span class="ub-value">' + p.input_tokens + '</span></span>');
    if (p.output_tokens) items.push('<span class="ub-item"><span class="ub-label">输出</span><span class="ub-value">' + p.output_tokens + '</span></span>');
    if (p.duration_ms) items.push('<span class="ub-item"><span class="ub-label">耗时</span><span class="ub-value">' + (p.duration_ms / 1000).toFixed(1) + 's</span></span>');
    if (p.tokens_per_sec) items.push('<span class="ub-item"><span class="ub-label">速度</span><span class="ub-value">' + p.tokens_per_sec.toFixed(1) + ' tok/s</span></span>');
    bar.innerHTML = items.join('');
    state.currentAssistantEl.append(bar);
    scrollIfAtBottom();
  }

  return {
    appendStreamChunk,
    finalizeAssistantMessage,
    appendReasoning,
    createToolCard,
    updateToolCard,
    showUsageBar,
  };
}
