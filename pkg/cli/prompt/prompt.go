// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Provides minimal single-field interactive terminal prompts
// built on Bubble Tea, replacing the archived AlecAivazis/survey package.

// Package prompt implements small, single-field terminal prompts (e.g. for a
// URL or a password) on top of github.com/charmbracelet/bubbletea.
package prompt

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ErrAborted is returned by Ask when the user cancels the prompt, e.g. via
// Ctrl+C or Esc.
var ErrAborted = errors.New("prompt aborted")

// Required is a Validate function that rejects empty (or whitespace-only)
// input.
func Required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value is required")
	}
	return nil
}

// Config describes a single-field text prompt.
type Config struct {
	// Message is the question shown to the user.
	Message string

	// Help, if set, is displayed underneath the message.
	Help string

	// EchoMode controls how typed input is displayed, e.g.
	// textinput.EchoPassword to mask the input. Defaults to
	// textinput.EchoNormal.
	EchoMode textinput.EchoMode

	// Validate, if set, must pass before the input is accepted on submit.
	// The prompt will keep re-displaying the error and accepting input
	// until it does.
	Validate func(string) error
}

var (
	messageStyle = lipgloss.NewStyle().Bold(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// model is the Bubble Tea model backing Ask.
type model struct {
	cfg     Config
	input   textinput.Model
	initCmd tea.Cmd
	err     error
}

func newModel(cfg Config) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.EchoMode = cfg.EchoMode

	return model{cfg: cfg, input: ti, initCmd: ti.Focus()}
}

func (m model) Init() tea.Cmd {
	return m.initCmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
			m.err = ErrAborted
			return m, tea.Quit
		case "enter":
			if m.cfg.Validate != nil {
				if err := m.cfg.Validate(m.input.Value()); err != nil {
					m.input.Err = err
					return m, nil
				}
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	var b strings.Builder

	fmt.Fprintln(&b, messageStyle.Render(m.cfg.Message))
	if m.cfg.Help != "" {
		fmt.Fprintln(&b, helpStyle.Render(m.cfg.Help))
	}
	b.WriteString(m.input.View())
	if m.input.Err != nil {
		fmt.Fprintf(&b, "\n%s", errorStyle.Render(m.input.Err.Error()))
	}

	return tea.NewView(b.String())
}

// Ask displays a single-field terminal prompt and returns the entered
// value. It returns ErrAborted if the user cancels the prompt.
func Ask(cfg Config) (string, error) {
	finalModel, err := tea.NewProgram(newModel(cfg)).Run()
	if err != nil {
		return "", err
	}

	m := finalModel.(model) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.err != nil {
		return "", m.err
	}

	return m.input.Value(), nil
}
