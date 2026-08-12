// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
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

		assert.DeepEqual(t, m.selectedOptions(), []string{"a", "c"})
	})

	t.Run("space toggles an option back off", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		m.Update(codeKeypress(tea.KeySpace))

		assert.Assert(t, !m.selected[0])
	})

	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(ctrlCKeypress())

		assert.Assert(t, m.aborted)
	})

	t.Run("esc aborts", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeyEscape))

		assert.Assert(t, m.aborted)
	})
}

func TestMultiSelectNoOptions(t *testing.T) {
	// No options is rejected before a Program is ever started, so this is
	// safe to call directly without a TTY.
	_, err := MultiSelect(t.Context(), MultiSelectConfig{})
	assert.ErrorIs(t, err, ErrAborted)
}

// TestMultiSelectModelFilter covers what a multiple-choice prompt has to
// get right about filtering that a single-choice one doesn't: selections
// outliving the filter used to find them. Narrowing belongs to the shared
// core, covered in list_test.go.
func TestMultiSelectModelFilter(t *testing.T) {
	options := []string{"kafka-broker-1", "redis-cache", "kafka-broker-2", "postgres-main"}

	t.Run("space toggles the highlighted match", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		typeString(m, "postgres")
		m.Update(codeKeypress(tea.KeySpace))

		assert.DeepEqual(t, m.selectedOptions(), []string{"postgres-main"})
	})

	// Selections are held by position in the full list, so narrowing to
	// find the next one cannot disturb the last.
	t.Run("selections survive a change of filter", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})

		typeString(m, "postgres")
		m.Update(codeKeypress(tea.KeySpace))

		m.Update(codeKeypress(tea.KeyEscape))
		typeString(m, "broker-2")
		m.Update(codeKeypress(tea.KeySpace))

		m.Update(codeKeypress(tea.KeyEnter))

		// In the order the options were listed, not the order they were
		// selected.
		assert.DeepEqual(t, m.selectedOptions(), []string{"kafka-broker-2", "postgres-main"})
	})

	t.Run("space does nothing while no option matches", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		typeString(m, "nonexistent")
		m.Update(codeKeypress(tea.KeySpace))

		assert.Equal(t, m.countSelected(), 0)
	})

	t.Run("enter confirms the selection even while filtered", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		typeString(m, "nonexistent")

		_, cmd := m.Update(codeKeypress(tea.KeyEnter))
		assert.Assert(t, cmd != nil, "enter should confirm")
		assert.DeepEqual(t, m.selectedOptions(), []string{"kafka-broker-1"})
	})

	t.Run("the footer counts selections that scrolled out of sight", func(t *testing.T) {
		m := newMultiSelectModel(MultiSelectConfig{Options: options})
		m.Update(codeKeypress(tea.KeySpace))
		typeString(m, "redis")

		view := m.View().Content
		assert.Assert(t, !strings.Contains(view, "kafka-broker-1"),
			"the selected option is still on screen, so the count is not what's under test")
		assert.Assert(t, cmp.Contains(view, "1 selected"))
	})
}

// TestMultiSelectModelToggleAfterScrolling covers what a long list means
// for MultiSelect rather than for the shared core: a toggle after
// scrolling has to land on the option's index in the full list, not its
// position in the window. Windowing is covered in list_test.go.
func TestMultiSelectModelToggleAfterScrolling(t *testing.T) {
	options := numberedLabels("template", 40)

	m := newMultiSelectModel(MultiSelectConfig{Options: options})
	for range len(options) {
		m.Update(codeKeypress(tea.KeyDown))
	}
	m.Update(codeKeypress(tea.KeySpace))

	assert.Assert(t, cmp.Contains(m.View().Content, "template-39"))
	assert.DeepEqual(t, m.selectedOptions(), []string{"template-39"})
}
