// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func TestNewPickerModel(t *testing.T) {
	options := []Option[string]{
		{Label: "current", Description: "cannot pick this", Disabled: true, Value: "current"},
		{Label: "older", Value: "older"},
	}

	m := newPickerModel("title", options)

	items := m.list.Items()
	if len(items) != len(options) {
		t.Fatalf("got %d items, want %d", len(items), len(options))
	}

	for i, listItem := range items {
		item, ok := listItem.(pickerItem[string])
		if !ok {
			t.Fatalf("item %d has type %T, want pickerItem[string]", i, listItem)
		}
		if item.label != options[i].Label {
			t.Errorf("item %d: label = %q, want %q", i, item.label, options[i].Label)
		}
		if item.disabled != options[i].Disabled {
			t.Errorf("item %d: disabled = %v, want %v", i, item.disabled, options[i].Disabled)
		}
		if item.value != options[i].Value {
			t.Errorf("item %d: value = %q, want %q", i, item.value, options[i].Value)
		}
	}
}

func TestPickerModel(t *testing.T) {
	type item struct{ n int }

	newOptions := func() []Option[item] {
		return []Option[item]{
			{Label: "current", Description: "cannot pick", Disabled: true, Value: item{1}},
			{Label: "older", Value: item{2}},
			{Label: "oldest", Value: item{3}},
		}
	}

	t.Run("enter picks the highlighted option", func(t *testing.T) {
		m := newPickerModel("Choose", newOptions())
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if m.chosen == nil {
			t.Fatal("chosen = nil, want a picked item")
		}
		if m.chosen.value.n != 2 {
			t.Errorf("chosen.value = %+v, want {n:2}", m.chosen.value)
		}
	})

	t.Run("enter on a disabled option does not pick it", func(t *testing.T) {
		m := newPickerModel("Choose", newOptions())
		m.Update(codeKeypress(tea.KeyEnter))

		if m.chosen != nil {
			t.Fatalf("chosen = %+v, want nil (option is disabled)", m.chosen)
		}

		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if m.chosen == nil {
			t.Fatal("chosen = nil, want a picked item after moving off the disabled option")
		}
		if m.chosen.value.n != 2 {
			t.Errorf("chosen.value = %+v, want {n:2}", m.chosen.value)
		}
	})

	t.Run("ctrl+c cancels without picking", func(t *testing.T) {
		m := newPickerModel("Choose", newOptions())
		m.Update(ctrlCKeypress())

		if m.chosen != nil {
			t.Errorf("chosen = %+v, want nil", m.chosen)
		}
	})

	t.Run("esc cancels without picking", func(t *testing.T) {
		m := newPickerModel("Choose", newOptions())
		m.Update(codeKeypress(tea.KeyEscape))

		if m.chosen != nil {
			t.Errorf("chosen = %+v, want nil", m.chosen)
		}
	})

	t.Run("slash enters filtering mode", func(t *testing.T) {
		m := newPickerModel("Choose", newOptions())
		m.Update(keypress('/'))

		if got := m.list.FilterState(); got != list.Filtering {
			t.Errorf("FilterState() = %v, want Filtering", got)
		}
	})

	t.Run("ctrl+c quits even while filtering, unlike esc and enter", func(t *testing.T) {
		m := newPickerModel("Choose", newOptions())
		m.Update(keypress('/'))
		if got := m.list.FilterState(); got != list.Filtering {
			t.Fatalf("FilterState() = %v, want Filtering", got)
		}

		_, cmd := m.Update(ctrlCKeypress())
		if cmd == nil {
			t.Fatal("expected a quit command, got nil")
		}
		if msg := cmd(); msg != (tea.QuitMsg{}) {
			t.Errorf("cmd() = %#v, want tea.QuitMsg{}", msg)
		}
	})
}
