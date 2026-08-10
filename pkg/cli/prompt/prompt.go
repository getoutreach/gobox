// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Provides minimal interactive terminal prompts built on
// Bubble Tea, replacing the archived AlecAivazis/survey package.

// Package prompt implements small terminal prompts (text input, yes/no
// confirmation, single- and multi-choice selection, and a generic picker)
// on top of github.com/charmbracelet/bubbletea. Select and PickOne share
// one filterable, scrolling list; see PickOne.
package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ErrAborted is returned when the user cancels a prompt, e.g. via Ctrl+C
// or Esc. It is not used by PickOne, which signals cancellation through
// its own ok return value instead; see PickOne's doc comment.
var ErrAborted = errors.New("prompt aborted")

// Key names matched against tea.KeyPressMsg.String() across every model
// in this package.
const (
	keyCtrlC = "ctrl+c"
	keyEsc   = "esc"
	keyEnter = "enter"
)

// Required is a Validate function that rejects empty (or whitespace-only)
// input.
func Required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value is required")
	}
	return nil
}

var (
	messageStyle  = lipgloss.NewStyle().Bold(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))

	// confirmFocusedStyle and confirmBlurredStyle render the highlighted
	// and non-highlighted Yes/No buttons for Confirm.
	confirmFocusedStyle = lipgloss.NewStyle().Bold(true).Padding(0, 2).
				Background(lipgloss.Color("57")).Foreground(lipgloss.Color("230"))
	confirmBlurredStyle = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("240"))
)

// Config describes a single-field text prompt.
type Config struct {
	// Message is the question shown to the user.
	Message string

	// Help, if set, is displayed underneath the message.
	Help string

	// Default, if set, pre-fills the input. The user can edit or clear
	// it before submitting.
	Default string

	// EchoMode controls how typed input is displayed, e.g.
	// textinput.EchoPassword to mask the input. Defaults to
	// textinput.EchoNormal.
	EchoMode textinput.EchoMode

	// Validate, if set, must pass before the input is accepted on submit.
	// The prompt will keep re-displaying the error and accepting input
	// until it does.
	Validate func(string) error
}

// inputModel is the Bubble Tea model backing Ask.
type inputModel struct {
	cfg     Config
	input   textinput.Model
	initCmd tea.Cmd
	err     error
}

// newInputModel builds an inputModel for cfg, pre-filling Default (if
// set) and focusing the field.
func newInputModel(cfg Config) *inputModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.EchoMode = cfg.EchoMode
	if cfg.Default != "" {
		ti.SetValue(cfg.Default)
	}

	// Focus is taken on the model's own copy of the field, rather than on
	// ti inside the literal below: Go leaves the order of a composite
	// literal's field values and the calls among them unspecified, so
	// focusing ti there may or may not be reflected in the copy assigned
	// to input.
	m := &inputModel{cfg: cfg, input: ti}
	m.initCmd = m.input.Focus()

	return m
}

func (m *inputModel) Init() tea.Cmd {
	return m.initCmd
}

// Update handles a key press: ctrl+c and esc abort; enter submits,
// running Validate first and re-prompting on failure; anything else is
// forwarded to the underlying text input.
func (m *inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, keyEsc:
			m.err = ErrAborted
			return m, tea.Quit
		case keyEnter:
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

func (m *inputModel) View() tea.View {
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
// value. It returns ErrAborted if the user cancels the prompt, or if ctx
// is canceled while the prompt is running.
func Ask(ctx context.Context, cfg Config) (string, error) {
	finalModel, err := tea.NewProgram(newInputModel(cfg), tea.WithContext(ctx)).Run()
	if err != nil {
		return "", fmt.Errorf("running input prompt: %w", err)
	}

	m := finalModel.(*inputModel) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.err != nil {
		return "", m.err
	}

	return m.input.Value(), nil
}

// SelectConfig describes a single-choice prompt.
type SelectConfig struct {
	// Message is the question shown to the user.
	Message string

	// Help, if set, is displayed underneath the message.
	Help string

	// Options are the choices presented to the user. Move between them
	// with the up/down arrow keys, narrow them down by typing a filter,
	// and pick the highlighted one with Enter.
	Options []string
}

// Select displays a single-choice terminal prompt and returns the chosen
// option. It is the string-list special case of PickOne: the options
// scroll a window over the list, typing narrows them down to the ones
// containing what was typed, and Esc clears that filter again.
//
// It returns ErrAborted if the user cancels the prompt, if ctx is
// canceled while the prompt is running, or if no options are provided.
func Select(ctx context.Context, cfg SelectConfig) (string, error) {
	if len(cfg.Options) == 0 {
		return "", ErrAborted
	}

	options := make([]Option[string], len(cfg.Options))
	for i, opt := range cfg.Options {
		options[i] = Option[string]{Label: opt, Value: opt}
	}

	chosen, ok, err := pickOne(ctx, cfg.Message, cfg.Help, options)
	if err != nil {
		return "", err
	}
	// Select reports a cancel the way the rest of this package's prompts
	// do, rather than the way PickOne does; see PickOne's doc comment.
	if !ok {
		return "", ErrAborted
	}

	return chosen, nil
}

// MultiSelectConfig describes a multiple-choice prompt.
type MultiSelectConfig struct {
	// Message is the question shown to the user.
	Message string

	// Help, if set, is displayed underneath the message.
	Help string

	// Options are the choices presented to the user. Toggle an option
	// with Space, move with the up/down (or j/k) arrow keys, and confirm
	// the current set of selections with Enter.
	Options []string
}

// multiSelectModel is the Bubble Tea model backing MultiSelect.
type multiSelectModel struct {
	cfg      MultiSelectConfig
	cursor   int
	selected map[int]bool
	err      error
}

// newMultiSelectModel builds a multiSelectModel for cfg, with every
// option initially unselected.
func newMultiSelectModel(cfg MultiSelectConfig) *multiSelectModel {
	return &multiSelectModel{cfg: cfg, selected: make(map[int]bool, len(cfg.Options))}
}

func (m *multiSelectModel) Init() tea.Cmd {
	return nil
}

// Update handles a key press: ctrl+c and esc abort; up/k and down/j
// move the cursor; space toggles the highlighted option; enter
// confirms the current set of selections.
func (m *multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, keyEsc:
			m.err = ErrAborted
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.cfg.Options)-1 {
				m.cursor++
			}
		case "space":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case keyEnter:
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *multiSelectModel) View() tea.View {
	var b strings.Builder

	fmt.Fprintln(&b, messageStyle.Render(m.cfg.Message))
	if m.cfg.Help != "" {
		fmt.Fprintln(&b, helpStyle.Render(m.cfg.Help))
	}

	for i, opt := range m.cfg.Options {
		marker := "  "
		box := "[ ]"
		style := lipgloss.NewStyle()
		if m.selected[i] {
			box = "[x]"
		}
		if i == m.cursor {
			marker = "> "
			style = selectedStyle
		}
		fmt.Fprintf(&b, "%s%s %s\n", marker, box, style.Render(opt))
	}
	fmt.Fprintln(&b, helpStyle.Render("(space to toggle, enter to confirm)"))

	return tea.NewView(b.String())
}

// MultiSelect displays a multiple-choice terminal prompt and returns the
// chosen options, in the order they were listed in cfg.Options. It
// returns ErrAborted if the user cancels the prompt, if ctx is canceled
// while the prompt is running, or if no options are provided.
func MultiSelect(ctx context.Context, cfg MultiSelectConfig) ([]string, error) {
	if len(cfg.Options) == 0 {
		return nil, ErrAborted
	}

	finalModel, err := tea.NewProgram(newMultiSelectModel(cfg), tea.WithContext(ctx)).Run()
	if err != nil {
		return nil, fmt.Errorf("running multi-select prompt: %w", err)
	}

	m := finalModel.(*multiSelectModel) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.err != nil {
		return nil, m.err
	}

	var result []string
	for i, opt := range m.cfg.Options {
		if m.selected[i] {
			result = append(result, opt)
		}
	}

	return result, nil
}

// ConfirmConfig describes a yes/no confirmation prompt.
type ConfirmConfig struct {
	// Message is the question shown to the user.
	Message string

	// Help, if set, is displayed underneath the message.
	Help string

	// Default is the choice highlighted initially, and the one used when
	// the user presses Enter without typing y/n explicitly.
	Default bool
}

// confirmModel is the Bubble Tea model backing Confirm.
type confirmModel struct {
	cfg    ConfirmConfig
	cursor int // 0 = Yes, 1 = No
	err    error
}

// newConfirmModel builds a confirmModel for cfg, with the cursor
// starting on cfg.Default.
func newConfirmModel(cfg ConfirmConfig) *confirmModel {
	cursor := 1
	if cfg.Default {
		cursor = 0
	}

	return &confirmModel{cfg: cfg, cursor: cursor}
}

func (m *confirmModel) Init() tea.Cmd {
	return nil
}

// Update handles a key press. y/Y and n/N answer directly; left/h and
// right/l/tab move the highlight between Yes and No; enter answers with
// the highlighted choice.
func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, keyEsc:
			m.err = ErrAborted
			return m, tea.Quit
		case "y", "Y":
			m.cursor = 0
			return m, tea.Quit
		case "n", "N":
			m.cursor = 1
			return m, tea.Quit
		case "left", "h":
			m.cursor = 0
		case "right", "l", "tab":
			m.cursor = 1
		case keyEnter:
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *confirmModel) View() tea.View {
	yesStyle, noStyle := confirmBlurredStyle, confirmFocusedStyle
	if m.cursor == 0 {
		yesStyle, noStyle = confirmFocusedStyle, confirmBlurredStyle
	}

	var b strings.Builder
	fmt.Fprintln(&b, messageStyle.Render(m.cfg.Message))
	if m.cfg.Help != "" {
		fmt.Fprintln(&b, helpStyle.Render(m.cfg.Help))
	}
	fmt.Fprintf(&b, "%s %s\n", yesStyle.Render("Yes"), noStyle.Render("No"))

	return tea.NewView(b.String())
}

// Confirm displays a yes/no terminal prompt and returns the user's
// choice. Pressing Enter without typing y/n returns cfg.Default. It
// returns ErrAborted if the user cancels the prompt, or if ctx is
// canceled while the prompt is running.
func Confirm(ctx context.Context, cfg ConfirmConfig) (bool, error) {
	finalModel, err := tea.NewProgram(newConfirmModel(cfg), tea.WithContext(ctx)).Run()
	if err != nil {
		return false, fmt.Errorf("running confirmation prompt: %w", err)
	}

	m := finalModel.(*confirmModel) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.err != nil {
		return false, m.err
	}

	return m.cursor == 0, nil
}

// Option is one choice in a PickOne list.
type Option[T any] struct {
	// Label is the option's main text, and the text the list is filtered
	// against.
	Label string

	// Description is a second line of text under Label. On a Disabled
	// option it is instead shown only when picking that option is
	// attempted, explaining why it can't be.
	Description string

	// Disabled options show in the list, but the user cannot pick them.
	// Description, if set, explains why.
	Disabled bool

	// Value is the value PickOne returns for this option.
	Value T
}

// maxVisibleOptions caps how many options are on screen at once, matching
// the page size the archived survey package used. A longer list scrolls a
// window of this size rather than rendering every option: a list taller
// than the terminal pushes the prompt itself, and the options above the
// fold, out of view entirely.
const maxVisibleOptions = 7

// filterPrompt labels the field holding the filter text.
const filterPrompt = "filter: "

// listModel is the Bubble Tea model backing PickOne, and through it
// Select. It scrolls a window over the options matching the filter the
// user has typed.
type listModel[T any] struct {
	title   string
	help    string
	options []Option[T]

	// filter holds the text the options are narrowed down by. It is only
	// rendered once non-empty; until then the footer advertises it.
	filter  textinput.Model
	initCmd tea.Cmd

	// matches holds the indexes into options of the options matching
	// filter, in their original order, and every index while filter is
	// empty. Options are tracked by index, rather than by their position
	// in a narrowed-down copy, so that the option picked is always the
	// one the cursor was on.
	matches []int

	// cursor is the position in matches of the highlighted option, and
	// offset the position in matches of the first option rendered:
	// together they scroll a maxVisibleOptions-sized window over the
	// matches.
	cursor int
	offset int

	// status explains why the last Enter press didn't pick anything. It
	// is cleared by the next key press.
	status string

	// chosen is the option the user picked, and nil if they canceled the
	// prompt instead.
	chosen *Option[T]
}

// newListModel builds a listModel offering options under title, with
// every option initially matching and the filter field focused. help, if
// set, is shown under the title.
func newListModel[T any](title, help string, options []Option[T]) *listModel[T] {
	ti := textinput.New()
	ti.Prompt = filterPrompt

	m := &listModel[T]{title: title, help: help, options: options, filter: ti}
	m.initCmd = m.filter.Focus()
	m.applyFilter()

	return m
}

func (m *listModel[T]) Init() tea.Cmd {
	return m.initCmd
}

// Update handles a key press: ctrl+c cancels; esc clears the filter, or
// cancels if there is none; up and down move the cursor within the
// matching options; enter picks the highlighted option, unless it is
// disabled. Anything else is forwarded to the filter field.
func (m *listModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m.updateFilter(msg)
	}

	// Any key press means the user has moved on from whatever the last
	// one was refused for.
	m.status = ""

	switch keyMsg.String() {
	case keyCtrlC:
		return m, tea.Quit
	case keyEsc:
		// Esc backs out of filtering first, so that a filter matching
		// nothing can be undone without losing the prompt itself; only
		// an unfiltered list cancels.
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyFilter()
			return m, nil
		}

		return m, tea.Quit
	case "up", "ctrl+p":
		m.moveCursor(-1)
		return m, nil
	case "down", "ctrl+n":
		m.moveCursor(1)
		return m, nil
	case keyEnter:
		// Enter has no option to return while the filter matches
		// nothing, so it waits for the filter to be loosened instead of
		// picking something the cursor was never on.
		if len(m.matches) == 0 {
			return m, nil
		}

		option := m.options[m.matches[m.cursor]]
		if option.Disabled {
			m.status = option.Description
			if m.status == "" {
				m.status = "This option cannot be picked."
			}

			return m, nil
		}

		m.chosen = &option

		return m, tea.Quit
	}

	return m.updateFilter(msg)
}

// updateFilter forwards msg to the filter field, re-narrowing the
// options if it changed the filter text.
func (m *listModel[T]) updateFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	before := m.filter.Value()

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != before {
		m.applyFilter()
	}

	return m, cmd
}

// applyFilter re-narrows matches to the options whose Label contains the
// filter text, case-insensitively, and returns the window to the top of
// the list: the previously highlighted option may not have survived the
// change, and for freshly typed text the best match is as likely to be
// the first one as any other.
func (m *listModel[T]) applyFilter() {
	needle := strings.ToLower(m.filter.Value())

	m.matches = make([]int, 0, len(m.options))
	for i, option := range m.options {
		if needle == "" || strings.Contains(strings.ToLower(option.Label), needle) {
			m.matches = append(m.matches, i)
		}
	}

	m.cursor = 0
	m.offset = 0
}

// moveCursor moves the cursor delta options through the matching ones,
// clamped to the ends of the list, scrolling the visible window as far
// as it takes to keep the cursor inside it.
func (m *listModel[T]) moveCursor(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), max(len(m.matches)-1, 0))

	switch {
	case m.cursor < m.offset:
		m.offset = m.cursor
	case m.cursor >= m.offset+maxVisibleOptions:
		m.offset = m.cursor - maxVisibleOptions + 1
	}
}

func (m *listModel[T]) View() tea.View {
	var b strings.Builder

	fmt.Fprintln(&b, messageStyle.Render(m.title))
	if m.help != "" {
		fmt.Fprintln(&b, helpStyle.Render(m.help))
	}
	if m.filter.Value() != "" {
		fmt.Fprintln(&b, m.filter.View())
	}

	if len(m.matches) == 0 {
		fmt.Fprintln(&b, helpStyle.Render("(no options match the filter, esc to clear it)"))
		return tea.NewView(b.String())
	}

	end := min(m.offset+maxVisibleOptions, len(m.matches))
	for _, idx := range m.matches[m.offset:end] {
		m.writeOption(&b, m.options[idx], idx == m.matches[m.cursor])
	}

	fmt.Fprintln(&b, helpStyle.Render(m.footer(end)))
	if m.status != "" {
		fmt.Fprintln(&b, errorStyle.Render(m.status))
	}

	return tea.NewView(b.String())
}

// writeOption renders one option's line (or two, if it has a persistent
// description), marked as the highlighted one if highlighted is set.
func (m *listModel[T]) writeOption(b *strings.Builder, option Option[T], highlighted bool) {
	marker, style := "  ", lipgloss.NewStyle()
	switch {
	case highlighted:
		marker, style = "> ", selectedStyle
	case option.Disabled:
		// A disabled option is dimmed rather than hidden: it is part of
		// the list the user is reasoning about, just not pickable.
		style = helpStyle
	}
	fmt.Fprintf(b, "%s%s\n", marker, style.Render(option.Label))

	// A disabled option's Description is its "why can't I pick this"
	// message, shown only when picking it is attempted, so it isn't a
	// second line here.
	if option.Description != "" && !option.Disabled {
		fmt.Fprintf(b, "    %s\n", helpStyle.Render(option.Description))
	}
}

// footer renders the line under the options, which says how to work the
// prompt and, when they don't all fit on screen at once, how much of the
// list is showing.
func (m *listModel[T]) footer(end int) string {
	if len(m.matches) > maxVisibleOptions {
		return fmt.Sprintf("(showing %d-%d of %d, type to filter, enter to select)", m.offset+1, end, len(m.matches))
	}

	return "(up/down to move, type to filter, enter to select)"
}

// PickOne shows options in an interactive list, under the given title.
// Type to narrow the list down to the options whose Label contains what
// was typed, use the arrow keys to move, and press enter to pick the
// highlighted option. A list longer than a screenful scrolls a window
// over the options rather than rendering all of them. Disabled options
// cannot be picked.
//
// PickOne returns the Value of the picked option. ok is false if the
// user canceled instead of picking an option (via Esc or Ctrl+C); this
// is not reported as an error. Canceling ctx also cancels the picker;
// PickOne then returns a non-nil error.
func PickOne[T any](ctx context.Context, title string, options []Option[T]) (value T, ok bool, err error) {
	return pickOne(ctx, title, "", options)
}

// pickOne is PickOne with the help line that Select's config exposes and
// PickOne's arguments have no room for.
func pickOne[T any](ctx context.Context, title, help string, options []Option[T]) (value T, ok bool, err error) {
	var zero T

	finalModel, err := tea.NewProgram(newListModel(title, help, options), tea.WithContext(ctx)).Run()
	if err != nil {
		return zero, false, fmt.Errorf("running picker: %w", err)
	}

	m, modelOK := finalModel.(*listModel[T])
	if !modelOK {
		return zero, false, fmt.Errorf("picker returned model of type %T", finalModel)
	}
	if m.chosen == nil {
		return zero, false, nil
	}

	return m.chosen.Value, true, nil
}
