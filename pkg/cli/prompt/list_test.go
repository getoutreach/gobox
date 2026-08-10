// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// matchedLabels returns the Labels of the options l currently has
// matching its filter, in order.
func (l *optionList[T]) matchedLabels() []string {
	matched := make([]string, 0, len(l.matches))
	for _, idx := range l.matches {
		matched = append(matched, l.options[idx].Label)
	}

	return matched
}

// highlightedLabel returns the Label of the option l has highlighted, or
// "" if nothing matches its filter.
func (l *optionList[T]) highlightedLabel() string {
	option, _, ok := l.highlighted()
	if !ok {
		return ""
	}

	return option.Label
}

// numberedLabels builds n labels of the form "<prefix>-00".
func numberedLabels(prefix string, n int) []string {
	labels := make([]string, 0, n)
	for i := range n {
		labels = append(labels, fmt.Sprintf("%s-%02d", prefix, i))
	}

	return labels
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

	if got, want := m.matchedLabels(), []string{"current", "older"}; !slices.Equal(got, want) {
		t.Errorf("matched %v, want %v", got, want)
	}
}

func TestListModelCursor(t *testing.T) {
	options := stringOptions("a", "b", "c")

	for _, tt := range []struct {
		name string
		keys []tea.KeyPressMsg
		want string
	}{
		{name: "starts on the first option", want: "a"},
		{name: "down moves forward", keys: []tea.KeyPressMsg{codeKeypress(tea.KeyDown), codeKeypress(tea.KeyDown)}, want: "c"},
		{name: "up moves back", keys: []tea.KeyPressMsg{codeKeypress(tea.KeyDown), codeKeypress(tea.KeyUp)}, want: "a"},
		{name: "ctrl+n and ctrl+p move too", keys: []tea.KeyPressMsg{ctrlKeypress('n'), ctrlKeypress('n'), ctrlKeypress('p')}, want: "b"},
		{
			name: "stops at the last option",
			keys: []tea.KeyPressMsg{codeKeypress(tea.KeyDown), codeKeypress(tea.KeyDown), codeKeypress(tea.KeyDown), codeKeypress(tea.KeyDown)},
			want: "c",
		},
		{name: "stops at the first option", keys: []tea.KeyPressMsg{codeKeypress(tea.KeyUp), codeKeypress(tea.KeyUp)}, want: "a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newListModel("", "", options)
			for _, key := range tt.keys {
				m.Update(key)
			}

			if got := m.highlightedLabel(); got != tt.want {
				t.Errorf("highlighted = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("enter picks the highlighted option", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedValue(t, m); got != "b" {
			t.Errorf("picked = %q, want %q", got, "b")
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
// what makes a list too long to read usable.
func TestListModelFilter(t *testing.T) {
	options := stringOptions("kafka-broker-1", "redis-cache", "kafka-broker-2", "postgres-main")

	t.Run("typing narrows the options to those containing the text", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")

		want := []string{"kafka-broker-1", "kafka-broker-2"}
		if got := m.matchedLabels(); !slices.Equal(got, want) {
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

	// Picking by cursor position alone would return whichever option sits
	// at that index in the full list.
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
		if got := m.highlightedLabel(); got != "postgres-main" {
			t.Errorf("highlighted = %q, want %q", got, "postgres-main")
		}
	})

	// The filter is typed directly rather than opened with a key, so "/"
	// is filter text like any other character.
	t.Run("slash is filter text, not a mode switch", func(t *testing.T) {
		m := newListModel("", "", stringOptions("with/slash", "without"))
		m.Update(keypress('/'))

		if got, want := m.filter.Value(), "/"; got != want {
			t.Errorf("filter = %q, want %q", got, want)
		}
		if got := m.matchedLabels(); !slices.Equal(got, []string{"with/slash"}) {
			t.Errorf("matched %v, want [with/slash]", got)
		}
	})
}

// TestListModelScrolling covers the case the window exists for: more
// options than fit on screen, which rendered in full push the prompt out
// of the terminal.
func TestListModelScrolling(t *testing.T) {
	options := stringOptions(numberedLabels("instance", 40)...)

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

		// The explanation belongs to that key press, so the next one
		// clears it.
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
	// A Description is a second line under its option's label.
	t.Run("a description adds a line under its own option only", func(t *testing.T) {
		m := newListModel("title", "", []Option[string]{
			{Label: "aaa", Description: "d1", Value: "a"},
			{Label: "bbb", Value: "b"},
		})

		// Undescribed options sit on consecutive lines, asserted by the
		// sibling subtest, so only aaa's description can push bbb down.
		view := m.View().Content
		if got := lineIndexContaining(view, "bbb") - lineIndexContaining(view, "aaa"); got != 2 {
			t.Errorf("gap between the options = %d, want 2 (label, description, label):\n%s", got, view)
		}
	})

	// A disabled option's Description explains why it can't be picked and
	// is shown only when picking it is attempted. This is the shape of a
	// real PickOne call: one disabled "current" choice explaining itself,
	// and plain choices with no description.
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
// canceled or picked nothing.
func pickedValue[T any](t *testing.T, m *listModel[T]) T {
	t.Helper()

	if m.chosen == nil {
		var zero T
		t.Fatal("chosen = nil, want a picked option")

		return zero
	}

	return m.chosen.Value
}

func TestListModelPaging(t *testing.T) {
	options := stringOptions(numberedLabels("instance", 40)...)

	for _, tt := range []struct {
		name string
		keys []tea.KeyPressMsg
		want string
	}{
		{name: "pgdown moves a windowful on", keys: []tea.KeyPressMsg{codeKeypress(tea.KeyPgDown)}, want: "instance-07"},
		{
			name: "pgup moves a windowful back",
			keys: []tea.KeyPressMsg{codeKeypress(tea.KeyPgDown), codeKeypress(tea.KeyPgDown), codeKeypress(tea.KeyPgUp)},
			want: "instance-07",
		},
		{name: "end jumps to the last option", keys: []tea.KeyPressMsg{codeKeypress(tea.KeyEnd)}, want: "instance-39"},
		{
			name: "home jumps back to the first",
			keys: []tea.KeyPressMsg{codeKeypress(tea.KeyEnd), codeKeypress(tea.KeyHome)},
			want: "instance-00",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newListModel("", "", options)
			for _, key := range tt.keys {
				m.Update(key)
			}

			if got := m.highlightedLabel(); got != tt.want {
				t.Errorf("highlighted = %q, want %q", got, tt.want)
			}
			if view := m.View().Content; !strings.Contains(view, tt.want) {
				t.Errorf("highlighted option is not in view:\n%s", view)
			}
		})
	}

	// g and G page in vim and in bubbles/list, but typing filters here.
	t.Run("g is filter text, not a jump", func(t *testing.T) {
		m := newListModel("", "", stringOptions("alpha", "gamma"))
		m.Update(keypress('g'))

		if got, want := m.filter.Value(), "g"; got != want {
			t.Errorf("filter = %q, want %q", got, want)
		}
		if got := m.highlightedLabel(); got != "gamma" {
			t.Errorf("highlighted = %q, want %q", got, "gamma")
		}
	})
}

// TestListModelTruncation covers labels wider than the terminal: left
// whole they wrap, and a wrapped row is taller than the one line the
// window budgets for it.
func TestListModelTruncation(t *testing.T) {
	const label = "i-0123456789abcdef (some-very-long-service-name-here)"

	t.Run("a label is left alone until a resize arrives", func(t *testing.T) {
		m := newListModel("", "", stringOptions(label))

		if view := m.View().Content; !strings.Contains(view, label) {
			t.Errorf("label was truncated with no known width:\n%s", view)
		}
	})

	t.Run("a label wider than the terminal is truncated with an ellipsis", func(t *testing.T) {
		m := newListModel("", "", stringOptions(label))
		m.Update(tea.WindowSizeMsg{Width: 24, Height: 20})

		view := m.View().Content
		if strings.Contains(view, label) {
			t.Errorf("label was not truncated:\n%s", view)
		}
		if !strings.Contains(view, ellipsis) {
			t.Errorf("truncated label is not marked with an ellipsis:\n%s", view)
		}
		for _, line := range strings.Split(strings.TrimRight(view, "\n"), "\n") {
			if got := lipgloss.Width(line); got > 24 {
				t.Errorf("line is %d columns wide, want at most 24: %q", got, line)
			}
		}
	})

	t.Run("a label that fits is left whole", func(t *testing.T) {
		m := newListModel("", "", stringOptions("short"))
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})

		if view := m.View().Content; !strings.Contains(view, "short") {
			t.Errorf("label that fits was altered:\n%s", view)
		}
	})

	t.Run("a checkbox row leaves room for its box", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: []string{label}})
		m.Update(tea.WindowSizeMsg{Width: 30, Height: 20})

		for _, line := range strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n") {
			if got := lipgloss.Width(line); got > 30 {
				t.Errorf("line is %d columns wide, want at most 30: %q", got, line)
			}
		}
	})
}

// TestListModelClampsEveryLine covers the lines that aren't options: a
// message or footer wider than the terminal wraps just as a label does.
func TestListModelClampsEveryLine(t *testing.T) {
	m := newListModel("A question far longer than the terminal it is asked in", "help that is also too long", stringOptions("a", "b"))
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 20})

	for _, line := range strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n") {
		if got := lipgloss.Width(line); got > 20 {
			t.Errorf("line is %d columns wide, want at most 20: %q", got, line)
		}
	}
}
