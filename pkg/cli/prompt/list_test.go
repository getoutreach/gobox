// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
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

// pickedValue returns the Value m settled on, failing the test if it was
// canceled or picked nothing.
func pickedValue[T any](t *testing.T, m *listModel[T]) T {
	t.Helper()

	assert.Assert(t, m.chosen != nil, "chosen is nil, want a picked option")

	return m.chosen.Value
}

// assertLinesFit fails the test for every line of view wider than width.
func assertLinesFit(t *testing.T, view string, width int) {
	t.Helper()

	for _, line := range strings.Split(strings.TrimRight(view, "\n"), "\n") {
		assert.Assert(t, lipgloss.Width(line) <= width,
			"line is %d columns wide, want at most %d: %q", lipgloss.Width(line), width, line)
	}
}

func TestNewListModel(t *testing.T) {
	options := []Option[string]{
		{Label: "current", Description: "cannot pick this", Disabled: true, Value: "current"},
		{Label: "older", Value: "older"},
	}

	m := newListModel("title", "", options)

	assert.DeepEqual(t, m.matchedLabels(), []string{"current", "older"})
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

			assert.Equal(t, m.highlightedLabel(), tt.want)
		})
	}

	t.Run("enter picks the highlighted option", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Equal(t, pickedValue(t, m), "b")
	})
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

			assert.Equal(t, m.highlightedLabel(), tt.want)
			assert.Assert(t, cmp.Contains(m.View().Content, tt.want))
		})
	}

	// g and G page in vim and in bubbles/list, but typing filters here.
	t.Run("g is filter text, not a jump", func(t *testing.T) {
		m := newListModel("", "", stringOptions("alpha", "gamma"))
		m.Update(keypress('g'))

		assert.Equal(t, m.filter.Value(), "g")
		assert.Equal(t, m.highlightedLabel(), "gamma")
	})
}

func TestListModelCancel(t *testing.T) {
	options := stringOptions("a", "b")

	t.Run("ctrl+c cancels without picking", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(ctrlCKeypress())

		assert.Assert(t, cmp.Nil(m.chosen))
	})

	t.Run("esc cancels without picking", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyEscape))

		assert.Assert(t, cmp.Nil(m.chosen))
	})

	t.Run("ctrl+c cancels even with a filter typed, unlike esc", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "a")

		_, cmd := m.Update(codeKeypress(tea.KeyEscape))
		assert.Assert(t, cmp.Nil(cmd), "esc with a filter typed should clear the filter, not quit")

		typeString(m, "a")
		_, cmd = m.Update(ctrlCKeypress())
		assert.Assert(t, cmd != nil, "ctrl+c should quit")
		assert.Equal(t, cmd(), tea.Msg(tea.QuitMsg{}))
	})
}

// TestListModelFilter covers narrowing a list down by typing, which is
// what makes a list too long to read usable.
func TestListModelFilter(t *testing.T) {
	options := stringOptions("kafka-broker-1", "redis-cache", "kafka-broker-2", "postgres-main")

	t.Run("typing narrows the options to those containing the text", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")

		assert.DeepEqual(t, m.matchedLabels(), []string{"kafka-broker-1", "kafka-broker-2"})
	})

	t.Run("filtering is case-insensitive", func(t *testing.T) {
		m := newListModel("", "", stringOptions("Bento-Alpha", "bento-beta", "other"))
		typeString(m, "BENTO")

		assert.DeepEqual(t, m.matchedLabels(), []string{"Bento-Alpha", "bento-beta"})
	})

	// Picking by cursor position alone would return whichever option sits
	// at that index in the full list.
	t.Run("enter picks the highlighted match, not the option at its index", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Equal(t, pickedValue(t, m), "kafka-broker-2")
	})

	t.Run("enter does nothing while no option matches", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "nonexistent")
		_, cmd := m.Update(codeKeypress(tea.KeyEnter))

		assert.Assert(t, cmp.Nil(cmd), "the prompt should stay open")
		assert.Assert(t, cmp.Nil(m.chosen))
		assert.Assert(t, cmp.Contains(m.View().Content, "no options match"))
	})

	t.Run("esc clears the filter instead of cancelling", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka")
		m.Update(codeKeypress(tea.KeyEscape))

		assert.Assert(t, cmp.Nil(m.chosen))
		assert.Equal(t, m.filter.Value(), "")
		assert.Assert(t, cmp.Len(m.matches, len(options)))
	})

	t.Run("backspace re-widens the list", func(t *testing.T) {
		m := newListModel("", "", options)
		typeString(m, "kafka-broker-1")
		assert.Assert(t, cmp.Len(m.matches, 1))

		for range len("-1") {
			m.Update(codeKeypress(tea.KeyBackspace))
		}

		assert.Equal(t, m.filter.Value(), "kafka-broker")
		assert.DeepEqual(t, m.matchedLabels(), []string{"kafka-broker-1", "kafka-broker-2"})
	})

	t.Run("a narrowed cursor returns to the top of the list", func(t *testing.T) {
		m := newListModel("", "", options)
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		typeString(m, "postgres")

		assert.Equal(t, m.cursor, 0)
		assert.Equal(t, m.highlightedLabel(), "postgres-main")
	})

	// The filter is typed directly rather than opened with a key, so "/"
	// is filter text like any other character.
	t.Run("slash is filter text, not a mode switch", func(t *testing.T) {
		m := newListModel("", "", stringOptions("with/slash", "without"))
		m.Update(keypress('/'))

		assert.Equal(t, m.filter.Value(), "/")
		assert.DeepEqual(t, m.matchedLabels(), []string{"with/slash"})
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
		assert.Equal(t, strings.Count(view, "instance-"), maxVisibleOptions)
		assert.Assert(t, cmp.Contains(view, "instance-00"))
		assert.Assert(t, !strings.Contains(view, "instance-39"), "window is not at the top of the list")
	})

	t.Run("the window follows the cursor down the list", func(t *testing.T) {
		m := newListModel("", "", options)
		for range len(options) {
			m.Update(codeKeypress(tea.KeyDown))
		}
		assert.Equal(t, m.cursor, len(options)-1)

		view := m.View().Content
		assert.Assert(t, cmp.Contains(view, "instance-39"))
		assert.Assert(t, !strings.Contains(view, "instance-00"), "window did not scroll away from the top")
		assert.Equal(t, strings.Count(view, "instance-"), maxVisibleOptions)
	})

	t.Run("the window follows the cursor back up the list", func(t *testing.T) {
		m := newListModel("", "", options)
		for range len(options) {
			m.Update(codeKeypress(tea.KeyDown))
		}
		for range len(options) {
			m.Update(codeKeypress(tea.KeyUp))
		}

		assert.Equal(t, m.offset, 0)
		assert.Assert(t, cmp.Contains(m.View().Content, "instance-00"))
	})

	t.Run("the footer says how much of the list is showing", func(t *testing.T) {
		m := newListModel("", "", options)

		assert.Assert(t, cmp.Contains(m.View().Content, fmt.Sprintf("of %d", len(options))))
	})

	t.Run("a list that fits is rendered whole", func(t *testing.T) {
		short := options[:3]
		m := newListModel("", "", short)

		assert.Equal(t, strings.Count(m.View().Content, "instance-"), len(short))
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

		assert.Equal(t, pickedValue(t, m), item{2})
	})

	t.Run("enter on a disabled option does not pick it", func(t *testing.T) {
		m := newListModel("Choose", "", newOptions())
		m.Update(codeKeypress(tea.KeyEnter))
		assert.Assert(t, cmp.Nil(m.chosen), "the option is disabled")

		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Equal(t, pickedValue(t, m), item{2})
	})

	t.Run("a refused pick explains itself with the option's description", func(t *testing.T) {
		m := newListModel("Choose", "", newOptions())
		m.Update(codeKeypress(tea.KeyEnter))
		assert.Assert(t, cmp.Contains(m.View().Content, "cannot pick"))

		// The explanation belongs to that key press, so the next one
		// clears it.
		m.Update(codeKeypress(tea.KeyDown))
		assert.Assert(t, !strings.Contains(m.View().Content, "cannot pick"),
			"explanation outlived the key press it answered")
	})

	t.Run("a disabled option with no description is still refused", func(t *testing.T) {
		m := newListModel("Choose", "", []Option[item]{{Label: "nope", Disabled: true, Value: item{1}}})
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Assert(t, cmp.Nil(m.chosen))
		assert.Assert(t, cmp.Contains(m.View().Content, "cannot be picked"))
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
		gap := lineIndexContaining(view, "bbb") - lineIndexContaining(view, "aaa")
		assert.Equal(t, gap, 2, "want label, description, label:\n%s", view)
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
		assert.Assert(t, !strings.Contains(view, "already the current choice"),
			"a disabled option's description is rendered as a line")

		gap := lineIndexContaining(view, "oldest") - lineIndexContaining(view, "older")
		assert.Equal(t, gap, 1, "want consecutive lines:\n%s", view)
	})
}

func TestListModelHelp(t *testing.T) {
	m := newListModel("Choose one", "this is the help text", stringOptions("a", "b"))

	assert.Assert(t, cmp.Contains(m.View().Content, "this is the help text"))
}

// TestListModelTruncation covers labels wider than the terminal: left
// whole they wrap, and a wrapped row is taller than the one line the
// window budgets for it.
func TestListModelTruncation(t *testing.T) {
	const label = "i-0123456789abcdef (some-very-long-service-name-here)"

	t.Run("a label is left alone until a resize arrives", func(t *testing.T) {
		m := newListModel("", "", stringOptions(label))

		assert.Assert(t, cmp.Contains(m.View().Content, label))
	})

	t.Run("a label wider than the terminal is truncated with an ellipsis", func(t *testing.T) {
		m := newListModel("", "", stringOptions(label))
		m.Update(tea.WindowSizeMsg{Width: 24, Height: 20})

		view := m.View().Content
		assert.Assert(t, !strings.Contains(view, label), "label was not truncated")
		assert.Assert(t, cmp.Contains(view, ellipsis))
		assertLinesFit(t, view, 24)
	})

	t.Run("a label that fits is left whole", func(t *testing.T) {
		m := newListModel("", "", stringOptions("short"))
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})

		assert.Assert(t, cmp.Contains(m.View().Content, "short"))
	})

	t.Run("a checkbox row leaves room for its box", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: []string{label}})
		m.Update(tea.WindowSizeMsg{Width: 30, Height: 20})

		assertLinesFit(t, m.View().Content, 30)
	})
}

// TestListModelClampsEveryLine covers the lines that aren't options: a
// message or footer wider than the terminal wraps just as a label does.
func TestListModelClampsEveryLine(t *testing.T) {
	m := newListModel("A question far longer than the terminal it is asked in", "help that is also too long", stringOptions("a", "b"))
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 20})

	assertLinesFit(t, m.View().Content, 20)
}
