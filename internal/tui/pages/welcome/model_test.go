package welcome

import "testing"

func TestModelUpdateKey(t *testing.T) {
	m := New(Deps{Tr: func(key string) string { return key }})
	items := []Item{
		{LabelKey: "new", Action: ActionNew},
		{LabelKey: "resume", Action: ActionResume},
		{LabelKey: "help", Action: ActionHelp, Disabled: true},
	}
	m.SetItems(items, 80)
	if !m.HasItems() {
		t.Fatal("expected initialized menu")
	}

	if action, handled := m.UpdateKey("down", items); !handled || action != ActionNone {
		t.Fatalf("down = (%v, %v), want (%v, true)", action, handled, ActionNone)
	}
	if action, handled := m.UpdateKey("enter", items); !handled || action != ActionResume {
		t.Fatalf("enter on resume = (%v, %v), want (%v, true)", action, handled, ActionResume)
	}
	if action, handled := m.UpdateKey("down", items); !handled || action != ActionNone {
		t.Fatalf("down to disabled = (%v, %v), want (%v, true)", action, handled, ActionNone)
	}
	if action, handled := m.UpdateKey("enter", items); !handled || action != ActionNone {
		t.Fatalf("enter on disabled = (%v, %v), want (%v, true)", action, handled, ActionNone)
	}
	if _, handled := m.UpdateKey("x", items); handled {
		t.Fatal("unexpected handling for unrelated key")
	}
}

func TestSetItemsClampsCursor(t *testing.T) {
	m := New(Deps{Tr: func(key string) string { return key }})
	items := []Item{
		{LabelKey: "new", Action: ActionNew},
		{LabelKey: "resume", Action: ActionResume},
	}
	m.SetItems(items, 80)
	_, _ = m.UpdateKey("down", items)

	items = []Item{{LabelKey: "new", Action: ActionNew}}
	m.SetItems(items, 80)
	if action, handled := m.UpdateKey("enter", items); !handled || action != ActionNew {
		t.Fatalf("clamped enter = (%v, %v), want (%v, true)", action, handled, ActionNew)
	}
}
