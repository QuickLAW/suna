package chat

// ModelPickerRow 是 /model 浮层的最小展示行；Config 详情由 root adapter 注入。
type ModelPickerRow struct {
	Ref     string
	Summary string
	Mark    string
}

type ModelPickerLabels struct {
	Empty string
	Title string
	Help  string
}

type ModelPickerView struct {
	Empty    string
	Title    string
	Help     string
	Rows     []ModelPickerRow
	Selected int
	Width    int
	Visible  bool
}

func (m Model) ModelPickerView(rows []ModelPickerRow, labels ModelPickerLabels, width int) ModelPickerView {
	if len(rows) == 0 {
		return ModelPickerView{Empty: labels.Empty, Width: width, Visible: true}
	}
	selected := m.ModelPickerCursor
	if selected < 0 {
		selected = 0
	}
	if selected >= len(rows) {
		selected = len(rows) - 1
	}
	return ModelPickerView{Title: labels.Title, Help: labels.Help, Rows: rows, Selected: selected, Width: width, Visible: true}
}
