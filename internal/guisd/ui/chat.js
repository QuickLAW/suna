/* chat.js — Agent 聊天主模块：WebSocket 状态机、消息分发、输入控制、用户消息发送。
   流式 / 思考 / 工具卡片由 chat-stream.js 负责；
   通用消息节点 / Guard / Ask / GuardConfirm / Error / 欢迎页由 chat-cards.js 负责。
   依赖 core.js 的 state/$/escapeFlag（仅 state），markdown.js 由 stream 使用。 */

import { $, state } from './core.js';
import { getState, setState, subscribe } from './state.js';
import { createChatStream } from './chat-stream.js';
import { createChatCards } from './chat-cards.js';

const chatMessages = $('chatMessages');
const chatInput = $('chatInput');
const sendBtn = $('sendBtn');
const cancelBtn = $('cancelBtn');

let agentWS = null;
let chatReqId = 0;
let chatBusy = false;
let reconnectDelay = 3000;
const maxReconnectDelay = 60000;

// 共享状态：stream / cards 都从这里读写。
const sharedState = {
  // 当前正在流式输出的 assistant 消息节点（每条流消息开始时设置）。
  currentAssistantEl: null,
  // 思考过程起始时间。
  reasoningStartTime: null,
  // 工具调用卡片缓存：id -> { card, header, body, resultEl }。
  toolCards: new Map(),
  // daemon.status 返回的运行时状态。
  daemonStatus: null,
  // config.get / config.state 返回的配置快照。
  configState: null,
};

// pendingRequests id -> method，用于 response 分发。
const pendingRequests = new Map();

let userScrolledUp = false;

// ============= 工具函数 =============

function scrollIfAtBottom() {
  if (!userScrolledUp) chatMessages.scrollTop = chatMessages.scrollHeight;
}

function agentSend(method, params) {
  if (!agentWS || agentWS.readyState !== WebSocket.OPEN) return null;
  chatReqId++;
  pendingRequests.set(chatReqId, method);
  agentWS.send(JSON.stringify({ jsonrpc: '2.0', id: chatReqId, method, params }));
  return chatReqId;
}

// ============= 工厂注入：stream + cards =============
// cards 必须先于 stream 初始化：stream 的 addChatMsg 来自 cards。
const cards = createChatCards({ chatMessages, scrollIfAtBottom, agentSend, state: sharedState });
const stream = createChatStream({ chatMessages, scrollIfAtBottom, addChatMsg: cards.addChatMsg, state: sharedState });

// hasUsableModel 暴露给 main.js（欢迎页 / 输入框判断）。
function hasUsableModel() { return cards.hasUsableModel(); }

// ============= WebSocket 状态机 =============

export function connectChat() {
  if (!state.daemonConnected) return;
  if (agentWS && (agentWS.readyState === WebSocket.OPEN || agentWS.readyState === WebSocket.CONNECTING)) return;
  const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  agentWS = new WebSocket(wsProto + '//' + location.host + '/api/agent');

  agentWS.onopen = () => {
    reconnectDelay = 3000;
    agentSend('daemon.status', null);
    agentSend('config.get', null);
    applyInputAvailability();
    renderWelcomeIfEmpty();
  };

  agentWS.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      handleAgentMessage(msg);
    } catch { /* 非 JSON 消息，静默忽略 */ }
  };

  agentWS.onclose = () => {
    if (chatBusy) setBusy(false);
    applyInputAvailability();
    const empty = $('chatEmpty');
    if (empty) {
      empty.style.display = 'flex';
      empty.innerHTML = '<div class="welcome"><div class="welcome-hint">与 daemon 断开连接，正在重连...</div></div>';
    }
    if (state.daemonConnected) {
      setTimeout(connectChat, reconnectDelay);
      reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
    }
  };

  agentWS.onerror = () => {};
}

// ============= 消息分发 =============

// handleAgentMessage 是所有 daemon 通知与响应的中央分发器。
function handleAgentMessage(msg) {
  // 通知（无 id）：按 method 分发。
  if (msg.method && msg.id == null) {
    const p = msg.params || {};
    switch (msg.method) {
      case 'agent.stream':
        if (p.done) {
          if (p.error) {
            cards.showErrorCard(p.chunk || '运行失败', { resume: p.resume_available });
          } else {
            stream.finalizeAssistantMessage();
          }
          setBusy(false);
        } else if (p.chunk) {
          stream.appendStreamChunk(p.chunk, p.error);
        }
        break;
      case 'agent.reasoning':
        stream.appendReasoning(p.chunk || '');
        break;
      case 'agent.tool_start':
        stream.createToolCard(p);
        sharedState.currentAssistantEl = null;
        break;
      case 'agent.tool_end':
        stream.updateToolCard(p);
        sharedState.currentAssistantEl = null;
        break;
      case 'agent.tool_guard':
        cards.showGuardCard(p);
        break;
      case 'agent.usage':
        stream.showUsageBar(p);
        break;
      case 'agent.ask_user':
        cards.showAskUserCard(p);
        break;
      case 'agent.guard_confirm':
        cards.showGuardConfirmCard(p);
        break;
      case 'daemon.full_status':
        updateDaemonStatus(p);
        break;
      case 'config.state':
        updateConfigState(p);
        break;
    }
    return;
  }
  // 响应（有 id）：按 pending method 分发结果或错误。
  if (msg.id != null) {
    const method = pendingRequests.get(msg.id);
    pendingRequests.delete(msg.id);
    if (msg.error) {
      handleRequestError(method, msg.error);
      return;
    }
    handleRequestResult(method, msg.result);
  }
}

function handleRequestResult(method, result) {
  if (!result) return;
  switch (method) {
    case 'daemon.status':
      updateDaemonStatus(result);
      break;
    case 'config.get':
      updateConfigState(result);
      break;
  }
}

// handleRequestError 把请求层错误反馈给用户。
function handleRequestError(method, err) {
  const msg = (err && err.message) || '请求失败';
  if (method === 'agent.sendMessage' || method === 'agent.resumeRun') {
    cards.showErrorCard(msg);
    setBusy(false);
  } else if (method === 'agent.askReply' || method === 'agent.guardReply') {
    cards.showErrorCard('交互回复失败：' + msg);
  }
  // 其余系统级请求（daemon.status / config.get 等）静默忽略。
}

// ============= 状态联动 =============

function setBusy(busy) {
  chatBusy = busy;
  cancelBtn.style.display = busy ? 'inline-block' : 'none';
  applyInputAvailability();
  if (!busy) chatInput.focus();
}

function sendMessage() {
  const text = chatInput.value.trim();
  if (!text || chatBusy) return;
  if (!hasUsableModel()) {
    cards.addChatMsg('user', text);
    chatInput.value = '';
    cards.showErrorCard('尚未配置或激活模型。请点击编辑区 `⚙ 设置` 标签添加并激活一个模型后再试。');
    return;
  }
  cards.addChatMsg('user', text);
  chatInput.value = '';
  userScrolledUp = false;
  sharedState.reasoningStartTime = null;
  setBusy(true);
  agentSend('agent.sendMessage', { parts: [{ type: 'text', text }] });
}

function cancelRun() {
  agentSend('agent.cancel', null);
}

function updateDaemonStatus(p) {
  if (!p) return;
  sharedState.daemonStatus = p;
  // 写入中央 store，供 status bar 等模块订阅。
  setState('daemon.status', p);
  const dot = $('daemonDot');
  const txt = $('daemonStatus');
  if (p.pid) {
    dot.className = 'status-dot online';
    const label = (p.provider || p.model) ? (p.provider || '') + '/' + (p.model || '') : 'Daemon 在线';
    txt.textContent = label;
    if (!state.daemonConnected) {
      state.daemonConnected = true;
      connectChat();
    }
  } else {
    dot.className = 'status-dot offline';
    txt.textContent = 'Daemon 离线';
  }
  if (p.uptime) {
    txt.title = 'PID: ' + p.pid + ' | 运行时间: ' + p.uptime + ' | 连接数: ' + p.connections;
  }
  renderWelcomeIfEmpty();
  applyInputAvailability();
}

function updateConfigState(p) {
  if (!p) return;
  sharedState.configState = p;
  // 写入中央 store，config.js 等模块通过 subscribe('config', ...) 自动收到通知。
  setState('config.snapshot', p);
  renderWelcomeIfEmpty();
  applyInputAvailability();
}

// applyInputAvailability 根据 daemon / 模型 / busy 状态更新输入框与发送按钮可用性。
export function applyInputAvailability() {
  const wsOpen = agentWS && agentWS.readyState === WebSocket.OPEN;
  if (!wsOpen) {
    chatInput.disabled = true;
    sendBtn.disabled = true;
    chatInput.placeholder = '等待 daemon 连接...';
    return;
  }
  if (chatBusy) {
    chatInput.disabled = true;
    sendBtn.disabled = true;
    chatInput.placeholder = '正在生成回复...';
    return;
  }
  if (!hasUsableModel()) {
    // 仍允许输入但发送时本地拦截，避免用户连话都打不下。
    chatInput.disabled = false;
    sendBtn.disabled = false;
    chatInput.placeholder = '尚未配置模型，先到编辑区 ⚙ 设置中添加';
    return;
  }
  chatInput.disabled = false;
  sendBtn.disabled = false;
  chatInput.placeholder = '输入消息，Enter 发送，Shift+Enter 换行';
}

// renderWelcomeIfEmpty 在聊天区为空时渲染欢迎页，否则隐藏。
export function renderWelcomeIfEmpty() {
  const empty = $('chatEmpty');
  if (!empty) return;
  const wsOpen = agentWS && agentWS.readyState === WebSocket.OPEN;
  const hasOtherChildren = Array.from(chatMessages.children).some(el => el !== empty);
  if (hasOtherChildren) {
    empty.style.display = 'none';
    return;
  }
  empty.style.display = 'flex';
  empty.innerHTML = cards.welcomeHTML(wsOpen);
}

// ============= 对外导出 =============

// sendAgentRequest 是 agentSend 的对外版本，供 config.js 等其他模块发起 JSON-RPC 请求。
export function sendAgentRequest(method, params) {
  return agentSend(method, params);
}

// onConfigState 保留向后兼容：通过 state.js subscribe 订阅 config.snapshot 变化。
export function onConfigState(cb) {
  return subscribe('config.snapshot', (val) => cb(val));
}

// initChat 绑定 UI 事件，启动时渲染一次欢迎页。
export function initChat() {
  chatInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });
  sendBtn.addEventListener('click', sendMessage);
  cancelBtn.addEventListener('click', cancelRun);
  chatMessages.addEventListener('scroll', () => {
    const atBottom = chatMessages.scrollHeight - chatMessages.scrollTop - chatMessages.clientHeight < 30;
    userScrolledUp = !atBottom;
  });
  renderWelcomeIfEmpty();
  applyInputAvailability();
}
