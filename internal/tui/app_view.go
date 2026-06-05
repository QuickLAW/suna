package tui

import (
	tea "charm.land/bubbletea/v2"
	uipage "github.com/alanchenchen/suna/internal/tui/pages/page"
)

func (t *TUI) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if t.copyMode {
		// 复制模式临时关闭鼠标捕获，把拖拽选择权还给终端；退出后恢复滚轮事件。
		v.MouseMode = tea.MouseModeNone
	}
	if !t.ready {
		v.SetContent(t.viewWelcome())
		return v
	}
	switch t.mode {
	case uipage.Welcome:
		v.SetContent(t.viewWelcome())
	case uipage.Config:
		v.SetContent(t.viewConfig())
	case uipage.Chat:
		v.SetContent(t.viewChat())
	case uipage.Help:
		v.SetContent(t.viewHelp())
	}
	return v
}
