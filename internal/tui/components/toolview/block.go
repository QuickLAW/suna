package toolview

import (
	"fmt"
	"strings"
)

func ParseSubtaskID(id string) (string, string) {
	const prefix = "spawn:"
	if !strings.HasPrefix(id, prefix) {
		return "", id
	}
	rest := strings.TrimPrefix(id, prefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", id
	}
	return parts[0], parts[1]
}

func VisibleEntries(block *Block) []*Entry {
	if block == nil {
		return nil
	}
	var entries []*Entry
	for _, id := range block.Order {
		te := block.Entries[id]
		if te == nil || te.ParentID != "" {
			continue
		}
		entries = append(entries, te)
		for _, childID := range block.Order {
			child := block.Entries[childID]
			if child == nil || child.ParentID != te.ID {
				continue
			}
			entries = append(entries, child)
		}
	}
	for _, id := range block.Order {
		te := block.Entries[id]
		if te == nil || te.ParentID == "" || block.Entries[te.ParentID] != nil {
			continue
		}
		entries = append(entries, te)
	}
	return entries
}

func ChangedFilePath(te *Entry) string {
	if !HasFileChange(te) {
		return ""
	}
	path, _ := te.Metadata["path"].(string)
	return strings.TrimSpace(path)
}

func HasFileChange(te *Entry) bool {
	if te == nil || te.Metadata == nil {
		return false
	}
	kind, _ := te.Metadata["kind"].(string)
	return kind == "file_change"
}

func ShouldShowGuardSummary(info *GuardInfo) bool {
	if info == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(info.Risk), "low") && strings.EqualFold(strings.TrimSpace(info.Decision), "approve") {
		return false
	}
	return true
}

func IsSubtask(te *Entry) bool {
	return te != nil && te.RawName == "spawn" && te.ParentID == ""
}

func IsSubtaskChild(te *Entry) bool {
	return te != nil && te.ParentID != ""
}

func BlockTitle(entries []*Entry, title string) string {
	parts := []string{title}
	if len(entries) > 0 {
		parts = append(parts, fmt.Sprintf("%d actions", len(entries)))
	}
	changedFiles := make(map[string]struct{})
	guards := 0
	for _, te := range entries {
		if path := ChangedFilePath(te); path != "" {
			changedFiles[path] = struct{}{}
		}
		if ShouldShowGuardSummary(te.Guard) {
			guards++
		}
	}
	if len(changedFiles) > 0 {
		parts = append(parts, fmt.Sprintf("%d files changed", len(changedFiles)))
	}
	if guards > 0 {
		parts = append(parts, fmt.Sprintf("%d guarded", guards))
	}
	return strings.Join(parts, " · ")
}
