package chat

func (m *Model) OpenModelPicker(modelRefs []string, activeRef string) {
	m.ModelPickerOpen = true
	m.ModelPickerCursor = 0
	for i, ref := range modelRefs {
		if ref == activeRef {
			m.ModelPickerCursor = i
			break
		}
	}
}

func (m *Model) CloseModelPicker() {
	m.ModelPickerOpen = false
}

func (m *Model) MoveModelPicker(delta, total int) {
	if total <= 0 {
		m.ModelPickerOpen = false
		m.ModelPickerCursor = 0
		return
	}
	m.ModelPickerCursor += delta
	if m.ModelPickerCursor < 0 {
		m.ModelPickerCursor = 0
	}
	if m.ModelPickerCursor >= total {
		m.ModelPickerCursor = total - 1
	}
}

func (m Model) SelectedModelRef(modelRefs []string) (string, bool) {
	if m.ModelPickerCursor < 0 || m.ModelPickerCursor >= len(modelRefs) {
		return "", false
	}
	return modelRefs[m.ModelPickerCursor], true
}

func ModelRefs[T interface{ Ref() string }](models []T) []string {
	refs := make([]string, 0, len(models))
	for _, model := range models {
		refs = append(refs, model.Ref())
	}
	return refs
}
