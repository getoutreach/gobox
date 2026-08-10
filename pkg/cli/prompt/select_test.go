// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// pickedOption returns the option m settled on, failing the test if it
// aborted, or ended without picking anything at all.
func pickedOption(t *testing.T, m *selectModel) string {
	t.Helper()

	if m.err != nil {
		t.Fatalf("aborted with err = %v", m.err)
	}
	if !m.picked {
		t.Fatal("picked nothing")
	}

	return m.chosen
}

func TestSelectModel(t *testing.T) {
	options := []string{"a", "b", "c"}

	t.Run("enter picks the initial cursor position", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedOption(t, m); got != "a" {
			t.Errorf("picked = %q, want %q", got, "a")
		}
	})

	t.Run("down moves the cursor before picking", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedOption(t, m); got != "c" {
			t.Errorf("picked = %q, want %q", got, "c")
		}
	})

	t.Run("cursor does not move past the last option", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		for range len(options) + 1 {
			m.Update(codeKeypress(tea.KeyDown))
		}

		if m.cursor != len(options)-1 {
			t.Errorf("cursor = %d, want %d", m.cursor, len(options)-1)
		}
	})

	t.Run("cursor does not move before the first option", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyUp))

		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		m.Update(ctrlCKeypress())

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})

	t.Run("esc aborts an unfiltered list", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyEscape))

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})
}

func TestSelectNoOptions(t *testing.T) {
	// No options is rejected before a Program is ever started, so this is
	// safe to call directly without a TTY.
	if _, err := Select(t.Context(), SelectConfig{}); !errors.Is(err, ErrAborted) {
		t.Errorf("err = %v, want ErrAborted", err)
	}
}

// TestSelectModelFilter covers narrowing a list down by typing, which is
// what makes a list too long to eyeball usable at all.
func TestSelectModelFilter(t *testing.T) {
	options := []string{"kafka-broker-1", "redis-cache", "kafka-broker-2", "postgres-main"}

	t.Run("typing narrows the options to those containing the text", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		typeString(m, "kafka")

		want := []string{"kafka-broker-1", "kafka-broker-2"}
		if got := matchedOptions(m); !slices.Equal(got, want) {
			t.Errorf("matches = %v, want %v", got, want)
		}
	})

	t.Run("filtering is case-insensitive", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: []string{"Bento-Alpha", "bento-beta", "other"}})
		typeString(m, "BENTO")

		if got := len(m.matches); got != 2 {
			t.Errorf("matched %d options, want 2", got)
		}
	})

	// The filtered list is a different list, so picking by cursor
	// position alone would return whichever option happens to sit at that
	// index in the full list instead of the highlighted one.
	t.Run("enter picks the highlighted match, not the option at its index", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		typeString(m, "kafka")
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := pickedOption(t, m); got != "kafka-broker-2" {
			t.Errorf("picked = %q, want %q", got, "kafka-broker-2")
		}
	})

	t.Run("enter does nothing while no option matches", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		typeString(m, "nonexistent")
		_, cmd := m.Update(codeKeypress(tea.KeyEnter))

		if cmd != nil {
			t.Error("enter returned a command, want nil (the prompt stays open)")
		}
		if m.picked {
			t.Errorf("picked = %q, want nothing picked", m.chosen)
		}
	})

	t.Run("esc clears the filter instead of aborting", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
		typeString(m, "kafka")
		m.Update(codeKeypress(tea.KeyEscape))

		if m.err != nil {
			t.Errorf("err = %v, want nil", m.err)
		}
		if m.filter.Value() != "" {
			t.Errorf("filter = %q, want empty", m.filter.Value())
		}
		if len(m.matches) != len(options) {
			t.Errorf("matched %d options, want all %d back", len(m.matches), len(options))
		}
	})

	t.Run("backspace re-widens the list", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
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
		m := newSelectModel(SelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		typeString(m, "postgres")

		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
		if got := pickAtCursor(m); got != "postgres-main" {
			t.Errorf("highlighted = %q, want %q", got, "postgres-main")
		}
	})
}

// TestSelectModelScrolling covers the case behind this behavior existing
// at all: a list of many more options than fit on screen, which without
// a window renders in full on every frame and scrolls the prompt itself
// out of the terminal.
func TestSelectModelScrolling(t *testing.T) {
	options := make([]string, 0, 40)
	for i := range 40 {
		options = append(options, fmt.Sprintf("instance-%02d", i))
	}

	t.Run("only a windowful of options is rendered", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Message: "pick one", Options: options})

		view := m.View().Content
		if got := strings.Count(view, "instance-"); got != maxVisibleOptions {
			t.Errorf("rendered %d options, want %d", got, maxVisibleOptions)
		}
		if !strings.Contains(view, "instance-00") || strings.Contains(view, "instance-39") {
			t.Errorf("window is not at the top of the list:\n%s", view)
		}
	})

	t.Run("the window follows the cursor down the list", func(t *testing.T) {
		m := newSelectModel(SelectConfig{Options: options})
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
		m := newSelectModel(SelectConfig{Options: options})
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
		m := newSelectModel(SelectConfig{Options: options})

		if want := fmt.Sprintf("of %d", len(options)); !strings.Contains(m.View().Content, want) {
			t.Errorf("view does not mention the full option count %q", want)
		}
	})

	t.Run("a list that fits is rendered whole", func(t *testing.T) {
		short := options[:3]
		m := newSelectModel(SelectConfig{Options: short})

		if got := strings.Count(m.View().Content, "instance-"); got != len(short) {
			t.Errorf("rendered %d options, want %d", got, len(short))
		}
	})
}

// matchedOptions returns the options m currently has matching its filter.
func matchedOptions(m *selectModel) []string {
	matched := make([]string, 0, len(m.matches))
	for _, idx := range m.matches {
		matched = append(matched, m.cfg.Options[idx])
	}

	return matched
}

// pickAtCursor returns the option m has highlighted.
func pickAtCursor(m *selectModel) string {
	if len(m.matches) == 0 {
		return ""
	}

	return m.cfg.Options[m.matches[m.cursor]]
}
