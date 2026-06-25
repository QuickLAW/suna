/* help.js — 帮助视图，渲染到 editor-body 中。
   与原 help overlay 文本保持一致，搬迁为编辑区特殊 tab 的内容。
   依赖 core.js 的 $。 */

import { $ } from './core.js';

// helpHTML 返回帮助视图的 HTML 字符串。整段内容静态，与原 help overlay 一致；
// 唯一调整是"快速开始"中关于"配置正在开发"那条改为指向新的前端设置面板。
function helpHTML() {
  return ''
    + '<div class="help-view">'
    + '  <div class="help-view-header">'
    + '    <div class="help-view-title">Suna · 帮助</div>'
    + '    <div class="help-view-sub">点击上方 <code>⚙ 设置</code> 标签可视化配置模型、安全模式与 Workspace。</div>'
    + '  </div>'
    + '  <div class="help-section">快速开始</div>'
    + '  <ul class="help-list">'
    + '    <li>在右侧聊天面板输入消息开始与 Suna 对话；流式回复、思考、工具调用都会展开为独立卡片。</li>'
    + '    <li>Agent 提出确认或追问时，请在卡片中点击按钮回复，否则当前运行会一直等待。</li>'
    + '    <li>模型、安全模式、Workspace 等可视化配置已迁移到本编辑区的 <code>⚙ 设置</code> 标签；TUI 的 <code>/config</code> 仍可用。</li>'
    + '  </ul>'
    + '  <div class="help-section">对话快捷键</div>'
    + '  <div class="help-row"><span class="hk">Enter</span><span class="hd">发送消息</span></div>'
    + '  <div class="help-row"><span class="hk">Shift+Enter</span><span class="hd">输入换行</span></div>'
    + '  <div class="help-row"><span class="hk">Esc</span><span class="hd">关闭浮层（聊天中按取消按钮中断当前运行）</span></div>'
    + '  <div class="help-section">编辑器 / 文件</div>'
    + '  <div class="help-row"><span class="hk">Ctrl+P</span><span class="hd">按文件名快速打开</span></div>'
    + '  <div class="help-row"><span class="hk">Ctrl+Shift+F</span><span class="hd">全局搜索文件内容</span></div>'
    + '  <div class="help-row"><span class="hk">Ctrl+W</span><span class="hd">关闭当前 Tab（未保存会询问）</span></div>'
    + '  <div class="help-row"><span class="hk">Ctrl+Tab / Ctrl+Shift+Tab</span><span class="hd">在已打开 Tab 之间循环切换</span></div>'
    + '  <div class="help-row"><span class="hk">Ctrl+S</span><span class="hd">保存当前文件</span></div>'
    + '  <div class="help-section">工具与安全</div>'
    + '  <ul class="help-list">'
    + '    <li>高风险工具调用会先发"安全确认"卡片，请审阅参数与原因后选择批准或拒绝。</li>'
    + '    <li>Workspace 已设置时，Guard 会拒绝目录外的本地文件和 exec 操作。</li>'
    + '  </ul>'
    + '  <div class="help-section">排查</div>'
    + '  <ul class="help-list">'
    + '    <li>发了消息没反应：先看头部 daemon 状态是否在线、模型是否激活；未配置模型会出现明确错误卡片。</li>'
    + '    <li>对话像被卡住：可能是 Agent 在等你回复确认/追问卡片，向上滚动找一下。</li>'
    + '  </ul>'
    + '</div>';
}

// renderHelpView 渲染帮助内容到 editor-body。调用方需先决定是否要保留
// 打开的文件 tab 状态（保存 / 恢复由 main.js 的切换逻辑负责）。
export function renderHelpView() {
  const body = $('editorBody');
  if (!body) return;
  body.innerHTML = helpHTML();
  body.scrollTop = 0;
}
