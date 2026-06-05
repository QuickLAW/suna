package chat

import "github.com/alanchenchen/suna/internal/protocol"

type SkillRowView struct {
	Skill    protocol.SkillInfo
	Selected bool
	Active   bool
	Issue    bool
}

type SkillsOverlayView struct {
	Rows    []SkillRowView
	Loading bool
	Empty   bool
	Error   string
	Active  int
	Issues  int
	Total   int
	Width   int
	Inner   int
	Height  int
}

func (m Model) SkillsOverlayView(width, overlayMaxHeight int) SkillsOverlayView {
	w := maxInt(48, minInt(82, width-4))
	inner := maxInt(28, w-8)
	bodyHeight := maxInt(4, minInt(14, overlayMaxHeight-8))
	active, issues := SkillSummaryCounts(m.Skills)
	rows := make([]SkillRowView, 0, len(m.Skills))
	for i, s := range m.Skills {
		rows = append(rows, SkillRowView{Skill: s, Selected: i == m.SkillsCursor, Active: SkillIsActive(s), Issue: SkillHasIssue(s)})
	}
	return SkillsOverlayView{
		Rows:    rows,
		Loading: m.SkillsLoading && len(m.Skills) == 0,
		Empty:   !m.SkillsLoading && len(m.Skills) == 0,
		Error:   m.SkillsError,
		Active:  active,
		Issues:  issues,
		Total:   len(m.Skills),
		Width:   w,
		Inner:   inner,
		Height:  bodyHeight,
	}
}
