package chat

const (
	InputMaxHeight        = 6
	MouseWheelDelta       = 3
	MaxCommandSuggestions = 4
)

type LayoutInput struct {
	Width            int
	Height           int
	InputHeight      int
	AttachmentHeight int
	SuggestionCount  int
	ConfirmDiscard   bool
}

type Layout struct {
	ViewportHeight int
	InputWidth     int
}

// ComputeLayout 只负责 Chat 页面尺寸策略；具体 Bubble components 由调用方设置。
func ComputeLayout(in LayoutInput) Layout {
	if in.Width == 0 || in.Height == 0 {
		return Layout{}
	}
	inputH := maxInt(1, in.InputHeight)
	suggestionH := 0
	if in.SuggestionCount > 0 {
		suggestionH = minInt(4, in.SuggestionCount) + 3
	}
	confirmH := 0
	if in.ConfirmDiscard {
		confirmH = 1
	}
	attachmentSeparatorH := 0
	if in.AttachmentHeight > 0 {
		attachmentSeparatorH = 1
	}
	fixedH := 6 + in.AttachmentHeight + attachmentSeparatorH + inputH + suggestionH + confirmH
	return Layout{ViewportHeight: maxInt(3, in.Height-fixedH), InputWidth: maxInt(20, in.Width-4)}
}

type ComposerHitInput struct {
	Height           int
	Y                int
	InputHeight      int
	AttachmentHeight int
	SuggestionCount  int
	ConfirmDiscard   bool
}

func MouseInComposer(in ComposerHitInput) bool {
	if in.Height <= 0 {
		return false
	}
	inputH := maxInt(1, in.InputHeight)
	suggestionH := 0
	if in.SuggestionCount > 0 {
		suggestionH = minInt(4, in.SuggestionCount) + 3
	}
	confirmH := 0
	if in.ConfirmDiscard {
		confirmH = 1
	}
	attachmentSeparatorH := 0
	if in.AttachmentHeight > 0 {
		attachmentSeparatorH = 1
	}
	composerStart := in.Height - (in.AttachmentHeight + attachmentSeparatorH + inputH + suggestionH + confirmH + 1)
	return in.Y >= composerStart
}

func AttachmentPanelHeight(panel string) int {
	if panel == "" {
		return 0
	}
	n := 1
	for _, r := range panel {
		if r == '\n' {
			n++
		}
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
