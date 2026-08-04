// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestConfirmModel(t *testing.T) {
	tests := []struct {
		name string
		cfg  ConfirmConfig
		keys []tea.KeyPressMsg
		want bool
	}{
		{name: "y confirms", cfg: ConfirmConfig{Message: "test"}, keys: []tea.KeyPressMsg{keypress('y')}, want: true},
		{name: "n declines", cfg: ConfirmConfig{Message: "test"}, keys: []tea.KeyPressMsg{keypress('n')}, want: false},
		{
			name: "enter alone uses Default=false",
			cfg:  ConfirmConfig{Message: "test", Default: false},
			keys: []tea.KeyPressMsg{codeKeypress(tea.KeyEnter)},
			want: false,
		},
		{
			name: "enter alone uses Default=true",
			cfg:  ConfirmConfig{Message: "test", Default: true},
			keys: []tea.KeyPressMsg{codeKeypress(tea.KeyEnter)},
			want: true,
		},
		{
			name: "left is idempotent: pressing it twice still lands on Yes",
			cfg:  ConfirmConfig{Message: "test", Default: false},
			keys: []tea.KeyPressMsg{
				codeKeypress(tea.KeyLeft), codeKeypress(tea.KeyLeft), codeKeypress(tea.KeyEnter),
			},
			want: true,
		},
		{
			name: "right is idempotent: pressing it twice still lands on No",
			cfg:  ConfirmConfig{Message: "test", Default: true},
			keys: []tea.KeyPressMsg{
				codeKeypress(tea.KeyRight), codeKeypress(tea.KeyRight), codeKeypress(tea.KeyEnter),
			},
			want: false,
		},
		{
			name: "tab moves off Default=true onto No",
			cfg:  ConfirmConfig{Message: "test", Default: true},
			keys: []tea.KeyPressMsg{codeKeypress(tea.KeyTab), codeKeypress(tea.KeyEnter)},
			want: false,
		},
		{
			name: "y overrides a highlighted No",
			cfg:  ConfirmConfig{Message: "test", Default: false},
			keys: []tea.KeyPressMsg{keypress('y')},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newConfirmModel(tt.cfg)
			for _, key := range tt.keys {
				m.Update(key)
			}

			if got := m.cursor == 0; got != tt.want {
				t.Errorf("confirmed = %v, want %v", got, tt.want)
			}
			if m.err != nil {
				t.Errorf("err = %v, want nil", m.err)
			}
		})
	}
}

func TestConfirmModelAbort(t *testing.T) {
	t.Run("ctrl+c aborts", func(t *testing.T) {
		m := newConfirmModel(ConfirmConfig{Message: "test"})
		m.Update(ctrlCKeypress())

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})

	t.Run("esc aborts", func(t *testing.T) {
		m := newConfirmModel(ConfirmConfig{Message: "test"})
		m.Update(codeKeypress(tea.KeyEscape))

		if !errors.Is(m.err, ErrAborted) {
			t.Errorf("err = %v, want ErrAborted", m.err)
		}
	})
}
