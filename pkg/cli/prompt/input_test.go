// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInputModel(t *testing.T) {
	t.Run("typed value is returned on enter", func(t *testing.T) {
		m := newInputModel(Config{Message: "test"})
		typeString(m, "hello")
		m.Update(codeKeypress(tea.KeyEnter))

		if got := m.input.Value(); got != "hello" {
			t.Errorf("Value() = %q, want %q", got, "hello")
		}
		if m.err != nil {
			t.Errorf("err = %v, want nil", m.err)
		}
	})

	t.Run("default is used unmodified on bare enter", func(t *testing.T) {
		m := newInputModel(Config{Message: "test", Default: "octocat"})
		m.Update(codeKeypress(tea.KeyEnter))

		if got := m.input.Value(); got != "octocat" {
			t.Errorf("Value() = %q, want %q", got, "octocat")
		}
	})

	t.Run("default can be edited before submit", func(t *testing.T) {
		m := newInputModel(Config{Message: "test", Default: "octocat"})
		typeString(m, "-fork")
		m.Update(codeKeypress(tea.KeyEnter))

		if got := m.input.Value(); got != "octocat-fork" {
			t.Errorf("Value() = %q, want %q", got, "octocat-fork")
		}
	})

	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := newInputModel(Config{Message: "test"})
		m.Update(ctrlCKeypress())

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})

	t.Run("esc aborts", func(t *testing.T) {
		m := newInputModel(Config{Message: "test"})
		m.Update(codeKeypress(tea.KeyEscape))

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})

	t.Run("validate rejects empty submission, then accepts once valid", func(t *testing.T) {
		m := newInputModel(Config{Message: "test", Validate: Required})
		m.Update(codeKeypress(tea.KeyEnter))

		if m.err != nil {
			t.Fatalf("err = %v, want nil (re-prompt, not abort)", m.err)
		}
		if m.input.Err == nil {
			t.Fatal("input.Err = nil, want a validation error displayed")
		}

		typeString(m, "ok")
		m.Update(codeKeypress(tea.KeyEnter))

		if m.err != nil {
			t.Errorf("err = %v, want nil", m.err)
		}
		if got := m.input.Value(); got != "ok" {
			t.Errorf("Value() = %q, want %q", got, "ok")
		}
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Required(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}
