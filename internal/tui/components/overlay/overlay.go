package overlay

import "strings"

// OverlayBlock 将浮层内容覆盖到基础内容顶部。
// Chat/Config 在迁移期间共用这套简单叠放逻辑，避免各页面重复实现。
func OverlayBlock(base, overlay string) string {
	if overlay == "" {
		return base
	}
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(strings.TrimRight(overlay, "\n"), "\n")
	for i, line := range overlayLines {
		if i < len(baseLines) {
			baseLines[i] = line
		} else {
			baseLines = append(baseLines, line)
		}
	}
	return strings.Join(baseLines, "\n")
}
