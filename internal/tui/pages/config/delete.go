package config

import "strings"

// DeleteOptions 返回删除确认框当前可用操作。
func DeleteOptions(cancel, deleteModel, deleteModelAndAPIKey string, offerAPIKey bool) []string {
	options := []string{cancel, deleteModel}
	if offerAPIKey {
		options = append(options, deleteModelAndAPIKey)
	}
	return options
}

type DeleteConfirmView struct {
	Ref      string
	Message  string
	Hint     string
	Help     string
	Options  []string
	Selected int
	MaxWidth int
}

func (m *Model) DeleteConfirmView(labels DeleteConfirmLabels, offerAPIKey bool, provider string, maxWidth int) DeleteConfirmView {
	options := DeleteOptions(labels.Cancel, labels.DeleteModel, labels.DeleteModelAndAPIKey, offerAPIKey)
	m.ClampDeleteCursor(len(options))
	view := DeleteConfirmView{
		Ref:      m.DeleteConfirm,
		Message:  strings.ReplaceAll(labels.Confirm, "{ref}", m.DeleteConfirm),
		Help:     labels.Help,
		Options:  options,
		Selected: m.DeleteCursor,
		MaxWidth: maxWidth,
	}
	if offerAPIKey && provider != "" {
		view.Hint = strings.ReplaceAll(labels.LastProviderKeyHint, "{provider}", provider)
	}
	return view
}

type DeleteConfirmLabels struct {
	Cancel               string
	DeleteModel          string
	DeleteModelAndAPIKey string
	Confirm              string
	LastProviderKeyHint  string
	Help                 string
}

func (m *Model) ClampDeleteCursor(optionCount int) {
	if optionCount <= 0 {
		m.DeleteCursor = 0
		return
	}
	if m.DeleteCursor < 0 {
		m.DeleteCursor = 0
	}
	if m.DeleteCursor >= optionCount {
		m.DeleteCursor = optionCount - 1
	}
}

func (m *Model) MoveDeleteCursor(delta, optionCount int) {
	if optionCount <= 0 {
		m.DeleteCursor = 0
		return
	}
	m.DeleteCursor = (m.DeleteCursor + delta + optionCount) % optionCount
}

func (m *Model) CancelDelete() {
	m.DeleteConfirm = ""
	m.DeleteCursor = 0
}

func (m *Model) ConfirmDelete(offerAPIKey bool) (ref string, deleteAPIKey bool, ok bool) {
	if m.DeleteConfirm == "" {
		return "", false, false
	}
	if m.DeleteCursor == 0 {
		m.CancelDelete()
		return "", false, false
	}
	ref = m.DeleteConfirm
	deleteAPIKey = offerAPIKey && m.DeleteCursor == 2
	m.CancelDelete()
	return ref, deleteAPIKey, true
}
