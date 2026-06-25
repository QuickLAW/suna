/* chat-cards.js — 聊天交互卡片：消息节点、Guard 决策、Usage、AskUser、GuardConfirm、Error、Welcome。
   工厂模式接收 chat.js 注入的依赖。
   依赖 core.js 的 escapeHtml。 */

import { escapeHtml } from './core.js';

// createChatCards 构造卡片渲染器。
// 依赖：
//   - chatMessages / scrollIfAtBottom: DOM 与自动滚动
//   - agentSend: 发送 JSON-RPC 请求
//   - state: 共享状态（daemonStatus / configState / currentAssistantEl / reasoningStartTime / toolCards）
export function createChatCards(deps) {
  const { chatMessages, scrollIfAtBottom, agentSend, state } = deps;

  // ===== 通用消息节点 =====

  function addChatMsg(role, content) {
    const empty = document.getElementById('chatEmpty');
    if (empty) empty.style.display = 'none';
    const msg = document.createElement('div');
    msg.className = 'chat-msg chat-msg-' + role;
    if (role === 'user') {
      const bubble = document.createElement('div');
      bubble.className = 'user-bubble';
      bubble.textContent = content;
      msg.append(bubble);
    } else if (role === 'assistant') {
      const head = document.createElement('div');
      head.className = 'assistant-head';
      head.append(document.createElement('div')).className = 'assistant-avatar';
      const name = document.createElement('span');
      name.className = 'assistant-name';
      name.textContent = 'Suna';
      head.append(name);
      const sub = document.createElement('span');
      sub.className = 'assistant-sub';
      sub.textContent = assistantSubLabel();
      head.append(sub);
      msg.append(head);
      const contentEl = document.createElement('div');
      contentEl.className = 'chat-msg-content';
      contentEl.textContent = content;
      msg.append(contentEl);
    } else {
      const roleEl = document.createElement('div');
      roleEl.className = 'chat-msg-role ' + role;
      const roleNames = { tool: '工具' };
      roleEl.textContent = roleNames[role] || role;
      const contentEl = document.createElement('div');
      contentEl.className = 'chat-msg-content';
      contentEl.textContent = content;
      msg.append(roleEl, contentEl);
    }
    chatMessages.append(msg);
    scrollIfAtBottom();
    if (role === 'user') {
      state.currentAssistantEl = null;
      state.reasoningStartTime = null;
    }
    return msg;
  }

  // assistantSubLabel 助手消息标题下的小字：当前模型 / 状态。
  function assistantSubLabel() {
    const m = (state.daemonStatus && state.daemonStatus.model) || (state.configState && state.configState.active_model);
    const p = (state.daemonStatus && state.daemonStatus.provider) || (state.configState && state.configState.models && state.configState.models[0] && state.configState.models[0].provider);
    if (m) return p ? p + ' / ' + m : m;
    return 'ready';
  }

  // ===== Guard 决策展示 =====

  function showGuardCard(p) {
    const card = document.createElement('div');
    const decision = p.decision || 'unknown';
    card.className = 'guard-card guard-' + decision;
    const risk = p.risk || 'unknown';
    const riskClass = risk === 'high' ? 'high' : risk === 'medium' ? 'medium' : 'low';

    let html = '<div class="guard-header">' +
      '<span>🛡 Guard</span>' +
      '<span class="gh-risk ' + riskClass + '">' + escapeHtml(risk) + '</span>' +
      '<span style="color:var(--text-dim)">' + escapeHtml(decision) + '</span>';
    if (p.source) html += '<span style="color:var(--text-faint);font-size:10px">(' + escapeHtml(p.source) + ')</span>';
    html += '</div>';
    if (p.reason) html += '<div class="guard-reason">' + escapeHtml(p.reason) + '</div>';
    if (p.suggestion) html += '<div class="guard-suggestion">💡 ' + escapeHtml(p.suggestion) + '</div>';
    card.innerHTML = html;

    const empty = document.getElementById('chatEmpty');
    if (empty) empty.style.display = 'none';
    chatMessages.append(card);
    scrollIfAtBottom();
  }

  // ===== AskUser 追问卡片 =====

  function showAskUserCard(p) {
    if (!p || !p.id) return;
    const card = document.createElement('div');
    card.className = 'ask-card';
    const id = p.id;

    let optionsHTML = '';
    if (Array.isArray(p.options) && p.options.length > 0) {
      optionsHTML = '<div class="ask-options">' + p.options.map((opt, i) =>
        '<button class="ask-option" data-i="' + i + '">' + escapeHtml(opt) + '</button>'
      ).join('') + '</div>';
    }
    const customHTML = p.allow_custom !== false
      ? '<div class="ask-custom">'
        + '<input class="ask-input" type="text" placeholder="或输入自定义回答..." />'
        + '<button class="chat-btn primary ask-submit">回复</button>'
        + '</div>'
      : '';

    card.innerHTML = ''
      + '<div class="ask-header">💬 Suna 正在等待你的回复</div>'
      + '<div class="ask-question">' + escapeHtml(p.question || '') + '</div>'
      + optionsHTML
      + customHTML
      + '<div class="ask-status" style="display:none"></div>';

    const empty = document.getElementById('chatEmpty');
    if (empty) empty.style.display = 'none';
    chatMessages.append(card);
    scrollIfAtBottom();

    const finalize = (answer) => {
      card.querySelectorAll('button').forEach(b => b.disabled = true);
      const input = card.querySelector('.ask-input');
      if (input) input.disabled = true;
      const status = card.querySelector('.ask-status');
      status.style.display = 'block';
      status.textContent = '已回复：' + answer;
      agentSend('agent.askReply', { id, answer });
    };

    card.querySelectorAll('.ask-option').forEach(btn => {
      btn.addEventListener('click', () => {
        const i = Number(btn.getAttribute('data-i'));
        finalize(p.options[i]);
      });
    });
    const submitBtn = card.querySelector('.ask-submit');
    const customInput = card.querySelector('.ask-input');
    if (submitBtn && customInput) {
      const submit = () => {
        const v = customInput.value.trim();
        if (v) finalize(v);
      };
      submitBtn.addEventListener('click', submit);
      customInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); }
      });
      customInput.focus();
    }
  }

  // ===== GuardConfirm 安全确认卡片 =====

  function showGuardConfirmCard(p) {
    if (!p || !p.id) return;
    const card = document.createElement('div');
    const risk = (p.risk || 'unknown').toLowerCase();
    const riskClass = risk === 'high' ? 'high' : risk === 'medium' ? 'medium' : 'low';
    card.className = 'guard-confirm-card risk-' + riskClass;

    let paramsBlock = '';
    if (p.params && Object.keys(p.params).length > 0) {
      paramsBlock = '<pre class="gc-params">' + escapeHtml(JSON.stringify(p.params, null, 2)) + '</pre>';
    }

    card.innerHTML = ''
      + '<div class="gc-header">'
      +   '<span>🛡 安全确认</span>'
      +   '<span class="gh-risk ' + riskClass + '">' + escapeHtml(p.risk || '未知') + '</span>'
      +   '<span class="gc-tool">' + escapeHtml(p.tool || '') + '</span>'
      + '</div>'
      + (p.reason ? '<div class="gc-reason">' + escapeHtml(p.reason) + '</div>' : '')
      + (p.suggestion ? '<div class="gc-suggestion">💡 ' + escapeHtml(p.suggestion) + '</div>' : '')
      + paramsBlock
      + '<div class="gc-actions">'
      +   '<button class="chat-btn ghost gc-reject">拒绝</button>'
      +   '<button class="chat-btn primary gc-approve">批准</button>'
      + '</div>'
      + '<div class="gc-status" style="display:none"></div>';

    const empty = document.getElementById('chatEmpty');
    if (empty) empty.style.display = 'none';
    chatMessages.append(card);
    scrollIfAtBottom();

    const finalize = (decision) => {
      card.querySelectorAll('button').forEach(b => b.disabled = true);
      const status = card.querySelector('.gc-status');
      status.style.display = 'block';
      status.textContent = decision === 'approve' ? '已批准' : '已拒绝';
      agentSend('agent.guardReply', { id: p.id, decision });
    };
    card.querySelector('.gc-approve').addEventListener('click', () => finalize('approve'));
    card.querySelector('.gc-reject').addEventListener('click', () => finalize('reject'));
  }

  // ===== Error 卡片 =====

  function showErrorCard(message, opts) {
    const card = document.createElement('div');
    card.className = 'error-card';
    let html = '<div class="ec-header">⚠ 错误</div>'
      + '<div class="ec-body">' + escapeHtml(message) + '</div>';
    if (opts && opts.resume) {
      html += '<div class="ec-hint">运行已中断，可重新发送消息或在 TUI 中按 Enter 恢复。</div>';
    }
    card.innerHTML = html;
    const empty = document.getElementById('chatEmpty');
    if (empty) empty.style.display = 'none';
    chatMessages.append(card);
    scrollIfAtBottom();
  }

  // ===== 欢迎页 =====

  function hasUsableModel() {
    if (state.daemonStatus && state.daemonStatus.provider && state.daemonStatus.model) return true;
    if (state.configState && Array.isArray(state.configState.models) && state.configState.models.length > 0) return true;
    return false;
  }

  function welcomeHTML(wsOpen) {
    const modelOk = hasUsableModel();
    const provider = (state.daemonStatus && state.daemonStatus.provider) || (state.configState && state.configState.models && state.configState.models[0] && state.configState.models[0].provider) || '';
    const model = (state.daemonStatus && state.daemonStatus.model) || (state.configState && state.configState.active_model) || '';
    const guardMode = (state.configState && state.configState.guard_mode) || '';
    const workspace = (state.configState && state.configState.workspace) || '';

    const statusItems = [];
    statusItems.push('<div class="welcome-row"><span class="wr-key">Daemon</span><span class="wr-val ' + (wsOpen ? 'ok' : 'bad') + '">' + (wsOpen ? '在线' : '离线') + '</span></div>');
    if (provider || model) {
      statusItems.push('<div class="welcome-row"><span class="wr-key">模型</span><span class="wr-val">' + escapeHtml((provider ? provider + ' / ' : '') + (model || '未激活')) + '</span></div>');
    } else {
      statusItems.push('<div class="welcome-row"><span class="wr-key">模型</span><span class="wr-val bad">未配置</span></div>');
    }
    if (guardMode) statusItems.push('<div class="welcome-row"><span class="wr-key">安全模式</span><span class="wr-val">' + escapeHtml(guardMode) + '</span></div>');
    if (workspace) statusItems.push('<div class="welcome-row"><span class="wr-key">Workspace</span><span class="wr-val mono">' + escapeHtml(workspace) + '</span></div>');

    const hint = !wsOpen
      ? '<div class="welcome-hint">等待与 daemon 建立连接...</div>'
      : (!modelOk
        ? '<div class="welcome-hint">尚未配置模型，请点击上方 <code>⚙ 设置</code> 标签，添加并激活一个模型，然后回到本页继续对话。</div>'
        : '<div class="welcome-hint">你已就绪，直接在下面输入消息开始与 Suna 对话。</div>');

    return ''
      + '<div class="welcome">'
      + '  <div class="welcome-brand">'
      + '    <div class="logo-dot"></div>'
      + '    <div class="welcome-title">Suna</div>'
      + '  </div>'
      + '  <div class="welcome-subtitle">你的有状态 AI 伙伴</div>'
      + '  <div class="welcome-status">' + statusItems.join('') + '</div>'
      + '  ' + hint
      + '  <div class="welcome-section">快速指引</div>'
      + '  <ul class="welcome-tips">'
      + '    <li>对话中支持流式回复、思考过程折叠和工具调用卡片，可点击展开查看详情。</li>'
      + '    <li>当 Agent 提出确认（高风险工具调用）或追问时，会以独立卡片出现，请直接点击按钮回复。</li>'
      + '    <li><b>Ctrl+P</b> 快速打开文件；<b>Ctrl+Shift+F</b> 全局搜索；<b>Ctrl+W</b> 关闭当前 Tab；<b>Ctrl+Tab</b> 在已打开 Tab 间切换。</li>'
      + '    <li>对话进行中可以点击右下角"取消"中断当前生成。</li>'
      + '    <li>模型、安全模式、Workspace 等配置已迁移到编辑区 <code>⚙ 设置</code> 标签；TUI 的 <code>/config</code> 仍可用。</li>'
      + '  </ul>'
      + '</div>';
  }

  return {
    addChatMsg,
    assistantSubLabel,
    showGuardCard,
    showAskUserCard,
    showGuardConfirmCard,
    showErrorCard,
    hasUsableModel,
    welcomeHTML,
  };
}
