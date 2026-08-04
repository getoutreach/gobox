// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"errors"
	"reflect"
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

		var got []string
		for i, opt := range m.cfg.Options {
			if m.selected[i] {
				got = append(got, opt)
			}
		}
		if want := []string{"a", "c"}; !reflect.DeepEqual(got, want) {
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
