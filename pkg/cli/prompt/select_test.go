// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSelectModel(t *testing.T) {
	options := []string{"a", "b", "c"}

	t.Run("enter picks the initial cursor position", func(t *testing.T) {
		m := &selectModel{cfg: SelectConfig{Options: options}}
		m.Update(codeKeypress(tea.KeyEnter))

		if got := m.cfg.Options[m.cursor]; got != "a" {
			t.Errorf("picked = %q, want %q", got, "a")
		}
	})

	t.Run("down moves the cursor before picking", func(t *testing.T) {
		m := &selectModel{cfg: SelectConfig{Options: options}}
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyDown))
		m.Update(codeKeypress(tea.KeyEnter))

		if got := m.cfg.Options[m.cursor]; got != "c" {
			t.Errorf("picked = %q, want %q", got, "c")
		}
	})

	t.Run("cursor does not move past the last option", func(t *testing.T) {
		m := &selectModel{cfg: SelectConfig{Options: options}}
		for range options {
			m.Update(codeKeypress(tea.KeyDown))
		}
		m.Update(codeKeypress(tea.KeyDown))

		if m.cursor != len(options)-1 {
			t.Errorf("cursor = %d, want %d", m.cursor, len(options)-1)
		}
	})

	t.Run("cursor does not move before the first option", func(t *testing.T) {
		m := &selectModel{cfg: SelectConfig{Options: options}}
		m.Update(codeKeypress(tea.KeyUp))

		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := &selectModel{cfg: SelectConfig{Options: options}}
		m.Update(ctrlCKeypress())

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})

	t.Run("esc aborts", func(t *testing.T) {
		m := &selectModel{cfg: SelectConfig{Options: options}}
		m.Update(codeKeypress(tea.KeyEscape))

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})
}

func TestSelectNoOptions(t *testing.T) {
	// No options is rejected before a Program is ever started, so this is
	// safe to call directly without a TTY.
	if _, err := Select(SelectConfig{}); !errors.Is(err, ErrAborted) {
		t.Errorf("err = %v, want ErrAborted", err)
	}
}
