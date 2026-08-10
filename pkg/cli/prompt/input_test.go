// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestInputModel(t *testing.T) {
	t.Run("typed value is returned on enter", func(t *testing.T) {
		m := newInputModel(Config{Message: "test"})
		typeString(m, "hello")
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Equal(t, m.input.Value(), "hello")
		assert.NilError(t, m.err)
	})

	t.Run("default is used unmodified on bare enter", func(t *testing.T) {
		m := newInputModel(Config{Message: "test", Default: "octocat"})
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Equal(t, m.input.Value(), "octocat")
	})

	t.Run("default can be edited before submit", func(t *testing.T) {
		m := newInputModel(Config{Message: "test", Default: "octocat"})
		typeString(m, "-fork")
		m.Update(codeKeypress(tea.KeyEnter))

		assert.Equal(t, m.input.Value(), "octocat-fork")
	})

	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := newInputModel(Config{Message: "test"})
		m.Update(ctrlCKeypress())

		assert.ErrorIs(t, m.err, ErrAborted)
	})

	t.Run("esc aborts", func(t *testing.T) {
		m := newInputModel(Config{Message: "test"})
		m.Update(codeKeypress(tea.KeyEscape))

		assert.ErrorIs(t, m.err, ErrAborted)
	})

	t.Run("validate rejects empty submission, then accepts once valid", func(t *testing.T) {
		m := newInputModel(Config{Message: "test", Validate: Required})
		_, cmd := m.Update(codeKeypress(tea.KeyEnter))

		assert.NilError(t, m.err, "want a re-prompt, not an abort")
		assert.Assert(t, cmp.Nil(cmd), "want the prompt to stay open")
		assert.Assert(t, m.input.Err != nil, "want a validation error displayed")

		typeString(m, "ok")
		_, cmd = m.Update(codeKeypress(tea.KeyEnter))

		assert.NilError(t, m.err)
		assert.Assert(t, cmd != nil, "want the valid value submitted")
		assert.Equal(t, m.input.Value(), "ok")
	})
}

func TestRequired(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty string is rejected", value: "", wantErr: true},
		{name: "whitespace-only is rejected", value: "   ", wantErr: true},
		{name: "non-empty value passes", value: "ok", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Required(tt.value)
			assert.Equal(t, err != nil, tt.wantErr, "Required(%q) = %v", tt.value, err)
		})
	}
}
