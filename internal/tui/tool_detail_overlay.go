package tui

import (
	"fmt"
	"strings"
)

func (t *TUI) renderToolDetailOverlay(width int) string {
	te := t.findTool(t.selectedToolID)
	if te == nil {
		return ""
	}
	w := max(44, min(104, width-4))
	bodyHeight := t.toolDetailBodyHeight()
	// 工具结果可能很长；详情面板走虚拟数据源，只渲染当前可见窗口。
	source := t.toolDetailLineSource(te)
	body, start, total := virtualScrollWindow(source, bodyHeight, &t.toolDetailScroll)
	lines := append([]string(nil), body...)
	lines = append(lines, "", styleDim.Render(t.toolDetailHelpText(start, bodyHeight, total)))
	return boxStyle.Width(w).Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func (t *TUI) toolDetailBodyHeight() int {
	return max(1, t.overlayMaxHeight()-7)
}

func (t *TUI) toolDetailInnerWidth() int {
	w := max(44, min(104, t.width-4))
	return max(24, w-8)
}

func (t *TUI) toolDetailLineSource(te *toolEntry) virtualLineSource {
	if te == nil {
		return sliceLineSource(nil)
	}
	idx, total := t.selectedToolPosition()
	return t.buildToolDetailLineSource(te, t.toolDetailInnerWidth(), idx, total)
}

func (t *TUI) buildToolDetailLineSource(te *toolEntry, inner, idx, total int) virtualLineSource {
	if te == nil {
		return sliceLineSource(nil)
	}
	var sections virtualSections
	appendLines := func(lines ...string) {
		sections = append(sections, sliceLineSource(lines))
	}
	appendWrapped := func(content string) {
		sections = append(sections, newWrappedLineSection(content, inner, styleToolDim))
	}
	title := t.tr("tui.tool.detail_title")
	if isSubtask(te) {
		title = t.tr("tui.tool.subtask_detail_title")
	} else if isSubtaskChild(te) {
		title = t.tr("tui.tool.subtask_tool_detail_title")
	}
	if total > 0 {
		title += fmt.Sprintf(" · %d/%d", idx+1, total)
	}
	appendLines(styleHL.Render(title))
	appendLines(styleDim.Render(t.tr("tui.tool.tool")+": ") + styleTool.Render(te.rawName))
	if strings.TrimSpace(te.intent) != "" {
		appendLines(styleDim.Render(t.tr("tui.tool.intent")))
		appendWrapped(te.intent)
	}
	if isSubtask(te) {
		t.appendSubtaskParamsVirtual(&sections, te, inner)
	} else if te.params != "" {
		appendLines("", styleDim.Render(t.tr("tui.tool.params")))
		appendWrapped(te.params)
	}
	if te.guard != nil {
		appendLines("", styleDim.Render(t.tr("tui.tool.guard")))
		appendLines(styleDim.Render(t.tr("tui.tool.guard.decision")) + " " + t.renderGuardDecisionBadge(te.guard))
		appendLines(styleDim.Render(t.tr("tui.tool.guard.risk")) + " " + t.renderRiskBadge(te.guard.risk))
		if te.guard.source != "" {
			appendLines(styleDim.Render(t.tr("tui.tool.guard.source")) + " " + styleToolDim.Render(te.guard.source))
		}
		if strings.TrimSpace(te.guard.reason) != "" {
			appendLines(styleDim.Render(t.tr("tui.tool.guard.reason")))
			appendWrapped(te.guard.reason)
		}
		if strings.TrimSpace(te.guard.suggestion) != "" {
			appendLines(styleDim.Render(t.tr("tui.tool.guard.suggestion")))
			appendWrapped(te.guard.suggestion)
		}
	}
	if te.result != "" {
		meta := t.tr("tui.tool.result")
		if te.resultBytes > 0 {
			meta += fmt.Sprintf(" · %d %s", te.resultBytes, t.tr("tui.tool.bytes"))
		}
		if te.resultTruncated {
			meta += " · " + t.tr("tui.tool.truncated")
		}
		appendLines("", styleDim.Render(meta))
		appendWrapped(te.result)
	}
	return sections
}

func (t *TUI) toolDetailHelpText(start, height, total int) string {
	idx, toolTotal := t.selectedToolPosition()
	var parts []string
	if total > height {
		parts = append(parts, fmt.Sprintf("PgUp/PgDn %s %d-%d/%d", t.tr("tui.overlay.scroll"), start+1, min(total, start+height), total))
	}
	if toolTotal > 1 {
		if idx > 0 {
			parts = append(parts, "↑ "+t.tr("tui.tool.prev"))
		}
		if idx < toolTotal-1 {
			parts = append(parts, "↓ "+t.tr("tui.tool.next"))
		}
	}
	parts = append(parts, "Ctrl+T/Esc "+t.tr("tui.tool.close"))
	return strings.Join(parts, " · ")
}

func (t *TUI) appendSubtaskParamsVirtual(sections *virtualSections, te *toolEntry, width int) {
	if sections == nil || te == nil || len(te.paramsRaw) == 0 {
		return
	}
	appendLines := func(lines ...string) {
		*sections = append(*sections, sliceLineSource(lines))
	}
	appendWrapped := func(content string) {
		*sections = append(*sections, newWrappedLineSection(content, width, styleToolDim))
	}
	if model, ok := te.paramsRaw["model"]; ok {
		appendLines("", styleDim.Render(t.tr("tui.tool.model")))
		appendLines(styleToolDim.Render(fmt.Sprintf("%v", model)))
	}
	if tools, ok := te.paramsRaw["tools"]; ok {
		appendLines("", styleDim.Render(t.tr("tui.tool.tools")))
		appendWrapped(fmt.Sprintf("%v", tools))
	}
	if task, ok := te.paramsRaw["task"]; ok {
		appendLines("", styleDim.Render(t.tr("tui.tool.task")))
		appendWrapped(fmt.Sprintf("%v", task))
	}
}

func splitWrapped(content string, width int, maxLines int) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		remaining := 0
		if maxLines > 0 {
			remaining = maxLines - len(out)
			if remaining <= 0 {
				return append(out, styleDim.Render("..."))
			}
		}
		for _, wrapped := range wrapLineLimit(line, width, remaining) {
			out = append(out, styleToolDim.Render(wrapped))
			if maxLines > 0 && len(out) >= maxLines {
				return append(out, styleDim.Render("..."))
			}
		}
	}
	return out
}
