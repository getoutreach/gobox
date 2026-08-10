// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stringOptions builds a list of Options whose Values are their Labels,
// which is the shape Select hands the model.
func stringOptions(labels ...string) []Option[string] {
	options := make([]Option[string], len(labels))
	for i, label := range labels {
		options[i] = Option[string]{Label: label, Value: label}
	}

	return options
}

// matchedLabels returns the Labels of the options l currently has
// matching its filter, in order.
func matchedLabels[T any](l *optionList[T]) []string {
	matched := make([]string, 0, len(l.matches))
	for _, idx := range l.matches {
		matched = append(matched, l.options[idx].Label)
	}

	return matched
}

// highlightedLabel returns the Label of the option l has highlighted, or
// "" if nothing matches its filter.
func highlightedLabel[T any](l *optionList[T]) string {
	option, _, ok := l.highlighted()
	if !ok {
		return ""
	}

	return option.Label
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

func TestNewListModel(t *testing.T) {
	options := []Option[string]{
		{Label: "current", Description: "cannot pick this", Disabled: true, Value: "current"},
		{Label: "older", Value: "older"},
	}

	m := newListModel("title", "", options)

	if got := len(m.matches); got != len(options) {
		t.Fatalf("matched %d options, want all %d", got, len(options))
	}
	if got, want := matchedLabels(&m.optionList), []string{"current", "older"}; !slices.Equal(got, want) {
		t.Errorf("matched %v, want %v", got, want)
	}
	if m.chosen != nil {
		t.Errorf("chosen = %+v, want nil before anything is picked", m.chosen)
	}
}

func TestListModelCursor(t *testing.T) {
	options := stringOptions("a", "b", "c")

	t.Run("enter picks the initial cursor position", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedValue(t, m); got != "a" {
			t.Errorf("picked = %q, want %q", got, "a")
		}
	})

	t.Run("down moves the cursor before picking", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedValue(t, m); got != "c" {
			t.Errorf("picked = %q, want %q", got, "c")
		}
	})

	t.Run("cursor does not move past the last option", func(t *testing.T) {
		m := newListModel("", "", options)
		for range len(options) + 1 {
			m.Update(codeKeypress(tea.KeyDown))
		}

		if m.cursor != len(options)-1 {
			t.Errorf("cursor = %d, want %d", m.cursor, len(options)-1)
		}
	})

	t.Run("cursor does not move before the first option", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyUp))

		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("ctrl+p and ctrl+n move the cursor too", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(ctrlKeypress('n'))
		m.Update(ctrlKeypress('n'))
		m.Update(ctrlKeypress('p'))

		if got := highlightedLabel(&m.optionList); got != "b" {
			t.Errorf("highlighted = %q, want %q", got, "b")
		}
	})
}

func TestListModelCancel(t *testing.T) {
	options := stringOptions("a", "b")

	t.Run("ctrl+c cancels without picking", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(ctrlCKeypress())

		if m.chosen != nil {
			t.Errorf("chosen = %+v, want nil", m.chosen)
		}
	})

	t.Run("esc cancels without picking", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyEscape))

		if m.chosen != nil {
			t.Errorf("chosen = %+v, want nil", m.chosen)
		}
	})

	t.Run("ctrl+c cancels even with a filter typed, unlike esc", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "a")

		_, cmd := m.Update(codeKeypress(tea.KeyEscape))
		if cmd != nil {
			t.Error("esc with a filter typed returned a command, want nil (it clears the filter)")
		}

		typeString(m, "a")
		_, cmd = m.Update(ctrlCKeypress())
		if cmd == nil {
			t.Fatal("expected a quit command, got nil")
		}
		if msg := cmd(); msg != (tea.QuitMsg{}) {
			t.Errorf("cmd() = %#v, want tea.QuitMsg{}", msg)
		}
	})
}

// TestListModelFilter covers narrowing a list down by typing, which is
// what makes a list too long to eyeball usable at all.
func TestListModelFilter(t *testing.T) {
	options := stringOptions("kafka-broker-1", "redis-cache", "kafka-broker-2", "postgres-main")

	t.Run("typing narrows the options to those containing the text", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")

		want := []string{"kafka-broker-1", "kafka-broker-2"}
		if got := matchedLabels(&m.optionList); !slices.Equal(got, want) {
			t.Errorf("matched %v, want %v", got, want)
		}
	})

	t.Run("filtering is case-insensitive", func(t *testing.T) {
		m := newListModel("", "", stringOptions("Bento-Alpha", "bento-beta", "other"))
		typeString(m, "BENTO")

		if got := len(m.matches); got != 2 {
			t.Errorf("matched %d options, want 2", got)
		}
	})

	// The filtered list is a different list, so picking by cursor
	// position alone would return whichever option happens to sit at that
	// index in the full list instead of the highlighted one.
	t.Run("enter picks the highlighted match, not the option at its index", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedValue(t, m); got != "kafka-broker-2" {
			t.Errorf("picked = %q, want %q", got, "kafka-broker-2")
		}
	})

	t.Run("enter does nothing while no option matches", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "nonexistent")
		_, cmd := m.Update(codeKeypress(tea.KeyEnter))

		if cmd != nil {
			t.Error("enter returned a command, want nil (the prompt stays open)")
		}
		if m.chosen != nil {
			t.Errorf("chosen = %+v, want nil", m.chosen)
		}
		if view := m.View().Content; !strings.Contains(view, "no options match") {
			t.Errorf("view does not say the filter matches nothing:\n%s", view)
		}
	})

	t.Run("esc clears the filter instead of cancelling", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")
		m.Update(codeKeypress(tea.KeyEscape))

		if m.chosen != nil {
			t.Errorf("chosen = %+v, want nil", m.chosen)
		}
		if m.filter.Value() != "" {
			t.Errorf("filter = %q, want empty", m.filter.Value())
		}
		if len(m.matches) != len(options) {
			t.Errorf("matched %d options, want all %d back", len(m.matches), len(options))
		}
	})

	t.Run("backspace re-widens the list", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka-broker-1")

		if got := len(m.matches); got != 1 {
			t.Fatalf("matched %d options on the full name, want 1", got)
		}

		for range len("-1") {
			m.Update(codeKeypress(tea.KeyBackspace))
		}

		if got, want := m.filter.Value(), "kafka-broker"; got != want {
			t.Fatalf("filter = %q, want %q", got, want)
		}
		if got := len(m.matches); got != 2 {
			t.Errorf("matched %d options, want both brokers back", got)
		}
	})

	t.Run("a narrowed cursor returns to the top of the list", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		typeString(m, "postgres")

		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
		if got := highlightedLabel(&m.optionList); got != "postgres-main" {
			t.Errorf("highlighted = %q, want %q", got, "postgres-main")
		}
	})

	// The filter is typed directly rather than opened with a key first,
	// so "/" is filter text like any other character.
	t.Run("slash is filter text, not a mode switch", func(t *testing.T) {
		m := newListModel("", "", stringOptions("with/slash", "without"))
		m.Update(keypress('/'))

		if got, want := m.filter.Value(), "/"; got != want {
			t.Errorf("filter = %q, want %q", got, want)
		}
		if got := matchedLabels(&m.optionList); !slices.Equal(got, []string{"with/slash"}) {
			t.Errorf("matched %v, want [with/slash]", got)
		}
	})
}

// TestListModelScrolling covers the case behind the window existing at
// all: many more options than fit on screen, which rendered in full push
// the prompt itself out of the terminal.
func TestListModelScrolling(t *testing.T) {
	labels := make([]string, 0, 40)
	for i := range 40 {
		labels = append(labels, fmt.Sprintf("instance-%02d", i))
	}
	options := stringOptions(labels...)

	t.Run("only a windowful of options is rendered", func(t *testing.T) {
		m := newListModel("pick one", "", options)

		view := m.View().Content
		if got := strings.Count(view, "instance-"); got != maxVisibleOptions {
			t.Errorf("rendered %d options, want %d", got, maxVisibleOptions)
		}
		if !strings.Contains(view, "instance-00") || strings.Contains(view, "instance-39") {
			t.Errorf("window is not at the top of the list:\n%s", view)
		}
	})

	t.Run("the window follows the cursor down the list", func(t *testing.T) {
		m := newListModel("", "", options)
		for range len(options) {
			m.Update(codeKeypress(tea.KeyDown))
		}

		if m.cursor != len(options)-1 {
			t.Fatalf("cursor = %d, want %d", m.cursor, len(options)-1)
		}

		view := m.View().Content
		if !strings.Contains(view, "instance-39") {
			t.Errorf("last option is not in view:\n%s", view)
		}
		if strings.Contains(view, "instance-00") {
			t.Errorf("window did not scroll away from the top:\n%s", view)
		}
		if got := strings.Count(view, "instance-"); got != maxVisibleOptions {
			t.Errorf("rendered %d options, want %d", got, maxVisibleOptions)
		}
	})

	t.Run("the window follows the cursor back up the list", func(t *testing.T) {
		m := newListModel("", "", options)
		for range len(options) {
			m.Update(codeKeypress(tea.KeyDown))
		}
		for range len(options) {
			m.Update(codeKeypress(tea.KeyUp))
		}

		if m.offset != 0 {
			t.Errorf("offset = %d, want 0", m.offset)
		}
		if !strings.Contains(m.View().Content, "instance-00") {
			t.Error("first option is not back in view")
		}
	})

	t.Run("the footer says how much of the list is showing", func(t *testing.T) {
		m := newListModel("", "", options)

		if want := fmt.Sprintf("of %d", len(options)); !strings.Contains(m.View().Content, want) {
			t.Errorf("view does not mention the full option count %q", want)
		}
	})

	t.Run("a list that fits is rendered whole", func(t *testing.T) {
		short := options[:3]
		m := newListModel("", "", short)

		if got := strings.Count(m.View().Content, "instance-"); got != len(short) {
			t.Errorf("rendered %d options, want %d", got, len(short))
		}
	})
}

func TestListModelDisabledOptions(t *testing.T) {
	type item struct{ n int }

	newOptions := func() []Option[item] {
		return []Option[item]{
			{Label: "current", Description: "cannot pick", Disabled: true, Value: item{1}},
			{Label: "older", Value: item{2}},
			{Label: "oldest", Value: item{3}},
		}
	}

	t.Run("enter picks the highlighted option", func(t *testing.T) {
		m := newListModel("Choose", "", newOptions())
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedValue(t, m); got.n != 2 {
			t.Errorf("picked = %+v, want {n:2}", got)
		}
	})

	t.Run("enter on a disabled option does not pick it", func(t *testing.T) {
		m := newListModel("Choose", "", newOptions())
		m.Update(codeKeypress(tea.KeyEnter))

		if m.chosen != nil {
			t.Fatalf("chosen = %+v, want nil (option is disabled)", m.chosen)
		}

		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedValue(t, m); got.n != 2 {
			t.Errorf("picked = %+v, want {n:2} after moving off the disabled option", got)
		}
	})

	t.Run("a refused pick explains itself with the option's description", func(t *testing.T) {
		m := newListModel("Choose", "", newOptions())
		m.Update(codeKeypress(tea.KeyEnter))

		if view := m.View().Content; !strings.Contains(view, "cannot pick") {
			t.Errorf("view does not explain why the option was refused:\n%s", view)
		}

		// The explanation is about the last key press, not a permanent
		// part of the list, so the next one clears it.
		m.Update(codeKeypress(tea.KeyDown))
		if view := m.View().Content; strings.Contains(view, "cannot pick") {
			t.Errorf("explanation outlived the key press it answered:\n%s", view)
		}
	})

	t.Run("a disabled option with no description is still refused", func(t *testing.T) {
		m := newListModel("Choose", "", []Option[item]{{Label: "nope", Disabled: true, Value: item{1}}})
		m.Update(codeKeypress(tea.KeyEnter))

		if m.chosen != nil {
			t.Fatalf("chosen = %+v, want nil", m.chosen)
		}
		if view := m.View().Content; !strings.Contains(view, "cannot be picked") {
			t.Errorf("view does not say the option is unpickable:\n%s", view)
		}
	})
}

func TestListModelDescriptions(t *testing.T) {
	// An option's Description is a second line under its label, so a
	// described option takes one line more than an undescribed one.
	t.Run("a description adds a line under its own option only", func(t *testing.T) {
		without := newListModel("title", "", stringOptions("aaa", "bbb"))
		with := newListModel("title", "", []Option[string]{
			{Label: "aaa", Description: "d1", Value: "a"},
			{Label: "bbb", Value: "b"},
		})

		gapWithout := lineIndexContaining(without.View().Content, "bbb") - lineIndexContaining(without.View().Content, "aaa")
		gapWith := lineIndexContaining(with.View().Content, "bbb") - lineIndexContaining(with.View().Content, "aaa")

		if gapWithout <= 0 || gapWith <= 0 {
			t.Fatalf("could not locate both options in either view: gapWithout=%d, gapWith=%d", gapWithout, gapWith)
		}
		if gapWith != gapWithout+1 {
			t.Errorf("gap between options with a description = %d, want %d (without: %d)", gapWith, gapWithout+1, gapWithout)
		}
	})

	// A disabled option's Description is its "why can't I pick this"
	// message, shown only when picking it is attempted. This is the
	// realistic shape of a PickOne call: one disabled "current" choice
	// explaining itself, and plain choices with no description at all.
	t.Run("a disabled option's description is not a line of its own", func(t *testing.T) {
		m := newListModel("title", "", []Option[string]{
			{Label: "current", Description: "already the current choice", Disabled: true, Value: "current"},
			{Label: "older", Value: "older"},
			{Label: "oldest", Value: "oldest"},
		})

		view := m.View().Content
		if strings.Contains(view, "already the current choice") {
			t.Errorf("disabled option's description is rendered as a line:\n%s", view)
		}

		olderIdx := lineIndexContaining(view, "older")
		oldestIdx := lineIndexContaining(view, "oldest")
		if olderIdx < 0 || oldestIdx < 0 {
			t.Fatalf("could not locate both options in view (older at %d, oldest at %d)", olderIdx, oldestIdx)
		}
		if gap := oldestIdx - olderIdx; gap != 1 {
			t.Errorf("gap between undescribed options = %d, want 1", gap)
		}
	})
}

func TestListModelHelp(t *testing.T) {
	m := newListModel("Choose one", "this is the help text", stringOptions("a", "b"))

	if view := m.View().Content; !strings.Contains(view, "this is the help text") {
		t.Errorf("view does not include the help text:\n%s", view)
	}
}

// pickedValue returns the Value m settled on, failing the test if it was
// canceled, or ended without picking anything at all.
func pickedValue[T any](t *testing.T, m *listModel[T]) T {
	t.Helper()

	if m.chosen == nil {
		var zero T
		t.Fatal("chosen = nil, want a picked option")

		return zero
	}

	return m.chosen.Value
}
