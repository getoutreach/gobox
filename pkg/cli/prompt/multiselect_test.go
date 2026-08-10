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

func TestMultiSelectModel(t *testing.T) {
	options := []string{"a", "b", "c"}

	t.Run("space toggles options on, in listed order", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeySpace))
		m.Update(codeKeypress(tea.KeyEnter))

		if got, want := m.selectedOptions(), []string{"a", "c"}; !slices.Equal(got, want) {
			t.Errorf("selected = %v, want %v", got, want)
		}
	})

	t.Run("space toggles an option back off", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		m.Update(codeKeypress(tea.KeySpace))

		if m.selected[0] {
			t.Error("selected[0] = true, want false after toggling twice")
		}
	})

	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(ctrlCKeypress())

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})

	t.Run("esc aborts", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyEscape))

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})
}

func TestMultiSelectNoOptions(t *testing.T) {
	// No options is rejected before a Program is ever started, so this is
	// safe to call directly without a TTY.
	if _, err := MultiSelect(t.Context(), MultiSelectConfig{}); !errors.Is(err, ErrAborted) {
		t.Errorf("err = %v, want ErrAborted", err)
	}
}

// TestMultiSelectModelFilter covers the filtering MultiSelect shares with
// the single-choice prompts, and the one thing only a multiple-choice
// prompt has to get right about it: a selection outliving the filter that
// was used to find it.
func TestMultiSelectModelFilter(t *testing.T) {
	options := []string{"kafka-broker-1", "redis-cache", "kafka-broker-2", "postgres-main"}

	t.Run("typing narrows the options", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		typeString(m, "kafka")

		want := []string{"kafka-broker-1", "kafka-broker-2"}
		if got := matchedLabels(&m.optionList); !slices.Equal(got, want) {
			t.Errorf("matched %v, want %v", got, want)
		}
	})

	t.Run("space toggles the highlighted match", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		typeString(m, "postgres")
		m.Update(codeKeypress(tea.KeySpace))

		if got, want := m.selectedOptions(), []string{"postgres-main"}; !slices.Equal(got, want) {
			t.Errorf("selected = %v, want %v", got, want)
		}
	})

	// Selections are held by position in the full list, so narrowing the
	// list down to find the next one can't disturb the last one.
	t.Run("selections survive a change of filter", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})

		typeString(m, "postgres")
		m.Update(codeKeypress(tea.KeySpace))

		m.Update(codeKeypress(tea.KeyEscape))
		typeString(m, "broker-2")
		m.Update(codeKeypress(tea.KeySpace))

		m.Update(codeKeypress(tea.KeyEnter))

		// Still in the order the options were listed in, not the order
		// they were selected in.
		if got, want := m.selectedOptions(), []string{"kafka-broker-2", "postgres-main"}; !slices.Equal(got, want) {
			t.Errorf("selected = %v, want %v", got, want)
		}
	})

	t.Run("space does nothing while no option matches", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		typeString(m, "nonexistent")
		m.Update(codeKeypress(tea.KeySpace))

		if got := m.selectedOptions(); len(got) != 0 {
			t.Errorf("selected = %v, want nothing", got)
		}
	})

	t.Run("enter confirms the selection even while filtered", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		typeString(m, "nonexistent")

		_, cmd := m.Update(codeKeypress(tea.KeyEnter))
		if cmd == nil {
			t.Fatal("expected a quit command, got nil")
		}
		if got, want := m.selectedOptions(), []string{"kafka-broker-1"}; !slices.Equal(got, want) {
			t.Errorf("selected = %v, want %v", got, want)
		}
	})

	t.Run("the footer counts selections that scrolled out of sight", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		typeString(m, "redis")

		view := m.View().Content
		if strings.Contains(view, "kafka-broker-1") {
			t.Fatalf("the selected option is still on screen, so the count isn't what's under test:\n%s", view)
		}
		if !strings.Contains(view, "1 selected") {
			t.Errorf("view does not count the off-screen selection:\n%s", view)
		}
	})
}

// TestMultiSelectModelScrolling covers the long lists that motivated
// windowing: rendered in full they push the prompt out of the terminal.
func TestMultiSelectModelScrolling(t *testing.T) {
	options := make([]string, 0, 40)
	for i := range 40 {
		options = append(options, fmt.Sprintf("template-%02d", i))
	}

	t.Run("only a windowful of options is rendered", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})

		view := m.View().Content
		if got := strings.Count(view, "template-"); got != maxVisibleOptions {
			t.Errorf("rendered %d options, want %d", got, maxVisibleOptions)
		}
		if want := fmt.Sprintf("of %d", len(options)); !strings.Contains(view, want) {
			t.Errorf("view does not mention the full option count %q", want)
		}
	})

	t.Run("the window follows the cursor down the list", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		for range len(options) {
			m.Update(codeKeypress(tea.KeyDown))
		}
		m.Update(codeKeypress(tea.KeySpace))

		view := m.View().Content
		if !strings.Contains(view, "template-39") {
			t.Errorf("last option is not in view:\n%s", view)
		}
		if strings.Contains(view, "template-00") {
			t.Errorf("window did not scroll away from the top:\n%s", view)
		}
		if got, want := m.selectedOptions(), []string{"template-39"}; !slices.Equal(got, want) {
			t.Errorf("selected = %v, want %v", got, want)
		}
	})
}
