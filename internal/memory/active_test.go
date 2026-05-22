package memory

import (
	"strings"
	"testing"
)

func TestSelectMemoriesKeepsLowPriorityPreferences(t *testing.T) {
	mems := []UserMemory{
		{ID: "preference:3", Kind: "preference", Content: "用户希望被称呼为「皮皮大人」，在对话中应始终使用该称呼。", Priority: 3, IsCore: true},
		{ID: "preference:2", Kind: "preference", Content: "用户喜欢丝袜大长腿美女，明确要求记住这一审美偏好。", Priority: 2},
	}

	selected := selectMemories(mems, "那你知道我喜欢？")
	if len(selected) != 2 {
		t.Fatalf("expected both active memories to be selected, got %d", len(selected))
	}
	var foundPreference bool
	for _, m := range selected {
		if strings.Contains(m.Content, "丝袜") {
			foundPreference = true
		}
	}
	if !foundPreference {
		t.Fatalf("expected low-priority preference to be selected: %#v", selected)
	}
}
