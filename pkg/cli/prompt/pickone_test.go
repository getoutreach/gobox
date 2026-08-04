// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"strings"
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

// lineIndexContaining returns the index of the first line in view
// containing substr, or -1 if none does.
func lineIndexContaining(view, substr string) int {
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, substr) {
			return i
		}
	}
	return -1
}

// TestPickerModelSkipsDescriptionLineWhenUnused checks that no option
// reserves a blank description line when none of the options have a
// Description. bubbles/list pads its View() to a fixed height, so the
// gap between two items' lines (rather than total line count) is what
// actually reveals whether a description line is being reserved.
func TestPickerModelSkipsDescriptionLineWhenUnused(t *testing.T) {
	without := newPickerModel("title", []Option[string]{
		{Label: "aaa", Value: "a"},
		{Label: "bbb", Value: "b"},
	})
	without.list.SetSize(80, 20)

	with := newPickerModel("title", []Option[string]{
		{Label: "aaa", Description: "d1", Value: "a"},
		{Label: "bbb", Value: "b"},
	})
	with.list.SetSize(80, 20)

	gapWithout := lineIndexContaining(without.list.View(), "bbb") - lineIndexContaining(without.list.View(), "aaa")
	gapWith := lineIndexContaining(with.list.View(), "bbb") - lineIndexContaining(with.list.View(), "aaa")

	if gapWithout <= 0 || gapWith <= 0 {
		t.Fatalf("could not locate both items in either view: gapWithout=%d, gapWith=%d", gapWithout, gapWith)
	}
	// Giving aaa a description adds exactly one line under it (its own
	// description), even though bbb still has none of its own.
	if gapWith != gapWithout+1 {
		t.Errorf("gap between items with a description = %d, want %d (without a description: %d)", gapWith, gapWithout+1, gapWithout)
	}
}

// TestPickerModelDisabledDescriptionDoesNotForceDescriptionLine checks
// that a disabled option's Description, used as its "why can't I pick
// this" status message, does not by itself force every other,
// undescribed option to reserve a blank description line. This is the
// realistic shape of a PickOne call: one disabled "current" choice
// explaining itself, and plain choices with no description at all.
func TestPickerModelDisabledDescriptionDoesNotForceDescriptionLine(t *testing.T) {
	m := newPickerModel("title", []Option[string]{
		{Label: "current", Description: "already the current choice", Disabled: true, Value: "current"},
		{Label: "older", Value: "older"},
		{Label: "oldest", Value: "oldest"},
	})
	m.list.SetSize(80, 20)

	view := m.list.View()
	olderIdx := lineIndexContaining(view, "older")
	oldestIdx := lineIndexContaining(view, "oldest")

	if olderIdx < 0 || oldestIdx < 0 {
		t.Fatalf("could not locate both items in view (older at %d, oldest at %d)", olderIdx, oldestIdx)
	}
	// "oldest" is listed right after "older". Normal item spacing (no
	// description line reserved) puts one blank line between them, for
	// a gap of 2; a reserved-but-empty description line would add one
	// more.
	if gap := oldestIdx - olderIdx; gap != 2 {
		t.Errorf("gap between undescribed items = %d, want 2 (no reserved description line)", gap)
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
