// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Provides minimal interactive terminal prompts built on
// Bubble Tea, replacing the archived AlecAivazis/survey package.

// Package prompt implements small terminal prompts (text input, yes/no
// confirmation, single- and multi-choice selection, and a filterable,
// generic picker) on top of github.com/charmbracelet/bubbletea.
package prompt

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
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

	return &inputModel{cfg: cfg, input: ti, initCmd: ti.Focus()}
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

// maxVisibleOptions caps how many of a Select's options are on screen at
// once, matching the page size the archived survey package used. A
// longer list scrolls a window of this size rather than rendering every
// option: a list taller than the terminal pushes the message, and the
// options above the fold, out of view entirely.
const maxVisibleOptions = 7

// filterPrompt labels the field holding a Select's filter text.
const filterPrompt = "filter: "

// selectModel is the Bubble Tea model backing Select.
type selectModel struct {
	cfg SelectConfig

	// filter holds the text the options are narrowed down by. It is only
	// rendered once non-empty; until then the footer advertises it.
	filter  textinput.Model
	initCmd tea.Cmd

	// matches holds the indexes into cfg.Options of the options matching
	// filter, in their original order, and every index while filter is
	// empty. Options are tracked by index, rather than by their position
	// in a narrowed-down copy, so that the option Select returns is
	// always the one the cursor was on.
	matches []int

	// cursor is the position in matches of the highlighted option, and
	// offset the position in matches of the first option rendered:
	// together they scroll a maxVisibleOptions-sized window over the
	// matches.
	cursor int
	offset int

	// chosen is the option the user picked, and picked records that they
	// picked one at all, as opposed to aborting the prompt.
	chosen string
	picked bool

	err error
}

// newSelectModel builds a selectModel for cfg, with every option
// initially matching and the filter field focused.
func newSelectModel(cfg SelectConfig) *selectModel {
	ti := textinput.New()
	ti.Prompt = filterPrompt

	m := &selectModel{cfg: cfg, filter: ti}
	m.initCmd = m.filter.Focus()
	m.applyFilter()

	return m
}

func (m *selectModel) Init() tea.Cmd {
	return m.initCmd
}

// Update handles a key press: ctrl+c aborts; esc clears the filter, or
// aborts if there is none; up and down move the cursor within the
// matching options; enter picks the highlighted option. Anything else is
// forwarded to the filter field.
func (m *selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m.updateFilter(msg)
	}

	switch keyMsg.String() {
	case keyCtrlC:
		m.err = ErrAborted
		return m, tea.Quit
	case keyEsc:
		// Esc backs out of filtering first, so that a filter matching
		// nothing can be undone without losing the prompt itself; only
		// an unfiltered list aborts.
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyFilter()
			return m, nil
		}

		m.err = ErrAborted
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

		m.chosen = m.cfg.Options[m.matches[m.cursor]]
		m.picked = true

		return m, tea.Quit
	}

	return m.updateFilter(msg)
}

// updateFilter forwards msg to the filter field, re-narrowing the
// options if it changed the filter text.
func (m *selectModel) updateFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	before := m.filter.Value()

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != before {
		m.applyFilter()
	}

	return m, cmd
}

// applyFilter re-narrows matches to the options containing the filter
// text, case-insensitively, and returns the window to the top of the
// list: the previously highlighted option may not have survived the
// change, and for freshly typed text the best match is as likely to be
// the first one as any other.
func (m *selectModel) applyFilter() {
	needle := strings.ToLower(m.filter.Value())

	m.matches = make([]int, 0, len(m.cfg.Options))
	for i, opt := range m.cfg.Options {
		if needle == "" || strings.Contains(strings.ToLower(opt), needle) {
			m.matches = append(m.matches, i)
		}
	}

	m.cursor = 0
	m.offset = 0
}

// moveCursor moves the cursor delta options through the matching ones,
// clamped to the ends of the list, scrolling the visible window as far
// as it takes to keep the cursor inside it.
func (m *selectModel) moveCursor(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), max(len(m.matches)-1, 0))

	switch {
	case m.cursor < m.offset:
		m.offset = m.cursor
	case m.cursor >= m.offset+maxVisibleOptions:
		m.offset = m.cursor - maxVisibleOptions + 1
	}
}

func (m *selectModel) View() tea.View {
	var b strings.Builder

	fmt.Fprintln(&b, messageStyle.Render(m.cfg.Message))
	if m.cfg.Help != "" {
		fmt.Fprintln(&b, helpStyle.Render(m.cfg.Help))
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
		marker := "  "
		style := lipgloss.NewStyle()
		if idx == m.matches[m.cursor] {
			marker = "> "
			style = selectedStyle
		}
		fmt.Fprintf(&b, "%s%s\n", marker, style.Render(m.cfg.Options[idx]))
	}
	fmt.Fprintln(&b, helpStyle.Render(m.footer(end)))

	return tea.NewView(b.String())
}

// footer renders the line under the options, which says how to work the
// prompt and, when they don't all fit on screen at once, how much of
// the list is showing.
func (m *selectModel) footer(end int) string {
	if len(m.matches) > maxVisibleOptions {
		return fmt.Sprintf("(showing %d-%d of %d, type to filter, enter to select)", m.offset+1, end, len(m.matches))
	}

	return "(up/down to move, type to filter, enter to select)"
}

// Select displays a single-choice terminal prompt and returns the chosen
// option. Options longer than a screenful scroll a window over the list;
// typing narrows the list down to the options containing what was typed,
// and Esc clears that filter again.
//
// It returns ErrAborted if the user cancels the prompt, if ctx is
// canceled while the prompt is running, or if no options are provided.
func Select(ctx context.Context, cfg SelectConfig) (string, error) {
	if len(cfg.Options) == 0 {
		return "", ErrAborted
	}

	finalModel, err := tea.NewProgram(newSelectModel(cfg), tea.WithContext(ctx)).Run()
	if err != nil {
		return "", fmt.Errorf("running selection prompt: %w", err)
	}

	m := finalModel.(*selectModel) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.err != nil {
		return "", m.err
	}
	if !m.picked {
		return "", ErrAborted
	}

	return m.chosen, nil
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
	// Label is the option's main text.
	Label string

	// Description is a second line of text under Label.
	Description string

	// Disabled options show in the list, but the user cannot pick them.
	// Description, if set, explains why.
	Disabled bool

	// Value is the value PickOne returns for this option.
	Value T
}

// pickerItem adapts an Option for use as a charm.land/bubbles/v2/list item.
type pickerItem[T any] struct {
	label       string
	description string
	disabled    bool
	value       T
}

// FilterValue returns the text the list filters against.
func (i pickerItem[T]) FilterValue() string { return i.label }

// Title returns the option's main line of text.
func (i pickerItem[T]) Title() string { return i.label }

// Description returns the option's second line of text.
func (i pickerItem[T]) Description() string { return i.description }

// pickerModel is the Bubble Tea model backing PickOne.
type pickerModel[T any] struct {
	list   list.Model
	chosen *pickerItem[T]
}

// newPickerModel builds a pickerModel listing options, in order, under
// title.
func newPickerModel[T any](title string, options []Option[T]) *pickerModel[T] {
	items := make([]list.Item, len(options))
	for i, o := range options {
		items[i] = pickerItem[T]{label: o.Label, description: o.Description, disabled: o.Disabled, value: o.Value}
	}

	// Only reserve a second line per item for descriptions if at least
	// one enabled option actually uses one as a persistent annotation.
	// A disabled option's Description is its "why can't I pick this"
	// status message instead (see Update below), shown only when
	// picking it is attempted, so it alone shouldn't force every other
	// item to render with a pointless blank line under it.
	hasDescription := slices.ContainsFunc(options, func(o Option[T]) bool {
		return !o.Disabled && o.Description != ""
	})
	delegate := list.NewDefaultDelegate()
	if !hasDescription {
		delegate.ShowDescription = false
	}

	// 80x20 is a placeholder size for the first frame. Bubble Tea sends
	// the real terminal size in a tea.WindowSizeMsg right after startup,
	// and Update resizes the list once that arrives.
	l := list.New(items, delegate, 80, 20)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	return &pickerModel[T]{list: l}
}

func (m *pickerModel[T]) Init() tea.Cmd {
	return nil
}

// Update handles list navigation and filtering, plus keys this model
// adds on top: ctrl+c always cancels, even mid-filter; esc cancels
// outside of filtering (bubbles/list's own esc-cancels-the-filter
// behavior takes over while filtering); and enter picks the highlighted
// option (or, if it's disabled, shows a status message instead of
// picking it).
func (m *pickerModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == keyCtrlC {
			return m, tea.Quit
		}

		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case keyEsc:
				return m, tea.Quit
			case keyEnter:
				item, ok := m.list.SelectedItem().(pickerItem[T])
				if !ok {
					return m, nil
				}
				if item.disabled {
					message := item.description
					if message == "" {
						message = "This option cannot be picked."
					}
					return m, m.list.NewStatusMessage(message)
				}
				m.chosen = &item
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *pickerModel[T]) View() tea.View {
	return tea.NewView(m.list.View())
}

// PickOne shows options in an interactive, filterable list, under the
// given title. Type "/" to filter the list, use the arrow keys to move,
// and press enter to pick the highlighted option. Disabled options
// cannot be picked.
//
// PickOne returns the Value of the picked option. ok is false if the
// user canceled instead of picking an option (via Esc or Ctrl+C); this
// is not reported as an error. Canceling ctx also cancels the picker;
// PickOne then returns a non-nil error.
func PickOne[T any](ctx context.Context, title string, options []Option[T]) (value T, ok bool, err error) {
	finalModel, err := tea.NewProgram(newPickerModel(title, options), tea.WithContext(ctx)).Run()
	if err != nil {
		var zero T
		return zero, false, fmt.Errorf("running picker: %w", err)
	}

	m, modelOK := finalModel.(*pickerModel[T])
	if !modelOK {
		var zero T
		return zero, false, fmt.Errorf("picker returned model of type %T", finalModel)
	}
	if m.chosen == nil {
		var zero T
		return zero, false, nil
	}

	return m.chosen.value, true, nil
}
