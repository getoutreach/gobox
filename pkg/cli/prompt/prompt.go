// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Provides minimal interactive terminal prompts built on
// Bubble Tea, replacing the archived AlecAivazis/survey package.

// Package prompt implements small terminal prompts (text input, yes/no
// confirmation, single- and multi-choice selection, and a generic picker)
// on top of github.com/charmbracelet/bubbletea. Select, MultiSelect and
// PickOne share one filterable, scrolling list; see PickOne.
package prompt

import (
	"cmp"
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

	// Focus is taken on the model's own copy of the field. Go leaves the
	// order of a composite literal's field values and the calls among
	// them unspecified, so focusing ti inside the literal below may not
	// be reflected in input.
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
// value. It returns ErrAborted if the user cancels the prompt, and a
// non-nil error if ctx is canceled while the prompt is running.
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
// option. It is the string-list case of PickOne: the options scroll a
// window, typing narrows them down, and Esc clears the filter.
//
// It returns ErrAborted if the user cancels the prompt or if no options
// are provided, and a non-nil error if ctx is canceled while the prompt
// is running.
func Select(ctx context.Context, cfg SelectConfig) (string, error) {
	if len(cfg.Options) == 0 {
		return "", ErrAborted
	}

	return pickOne(ctx, cfg.Message, cfg.Help, stringOptions(cfg.Options...))
}

// MultiSelectConfig describes a multiple-choice prompt.
type MultiSelectConfig struct {
	// Message is the question shown to the user.
	Message string

	// Help, if set, is displayed underneath the message.
	Help string

	// Options are the choices presented to the user. Move between them
	// with the up/down arrow keys, narrow them down by typing a filter,
	// toggle the highlighted one with Space, and confirm the current set
	// of selections with Enter.
	Options []string
}

// multiSelectModel is the Bubble Tea model backing MultiSelect.
type multiSelectModel struct {
	optionList[string]

	// selected says whether each option is selected, indexed as options
	// is.
	selected []bool
}

// newMultiSelectModel builds a multiSelectModel for cfg.
func newMultiSelectModel(cfg MultiSelectConfig) *multiSelectModel {
	options := stringOptions(cfg.Options...)

	return &multiSelectModel{
		optionList: newOptionList(cfg.Message, cfg.Help, options),
		selected:   make([]bool, len(options)),
	}
}

// Update handles the presses the list itself doesn't: Space toggles the
// highlighted option and Enter confirms the selections. Anything else
// goes to the filter field.
//
// Space toggles rather than filters, so a filter cannot contain a space.
// Matching one word of a multi-word option narrows the list just as well.
func (m *multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, m.updateFilter(msg)
	}

	key := keyMsg.String()
	switch m.handleKey(key) {
	case keyAborted:
		return m, tea.Quit
	case keyHandled:
		return m, nil
	case keyIgnored:
	}

	switch key {
	case "space":
		// A disabled option is not toggled. MultiSelect builds its
		// options from strings and never disables one, but a typed list
		// could.
		if option, index, ok := m.highlighted(); ok && !option.Disabled {
			m.selected[index] = !m.selected[index]
		}

		return m, nil
	case keyEnter:
		// Confirming with nothing selected is a valid answer.
		return m, tea.Quit
	}

	return m, m.updateFilter(msg)
}

func (m *multiSelectModel) View() tea.View {
	var b strings.Builder

	m.writeHeader(&b)
	m.writeOptions(&b, m.writeCheckbox)

	hints := make([]string, 0, 3)
	hints = append(hints, "space to toggle", "enter to confirm")
	// A selection filtered or scrolled out of sight still counts.
	if n := m.countSelected(); n > 0 {
		hints = append(hints, fmt.Sprintf("%d selected", n))
	}
	fmt.Fprintln(&b, helpStyle.Render(m.footer(hints...)))

	return tea.NewView(b.String())
}

// writeCheckbox renders one option's line and its selection box.
func (m *multiSelectModel) writeCheckbox(b *strings.Builder, index int, highlighted bool) {
	box := "[ ]"
	if m.selected[index] {
		box = "[x]"
	}

	marker, label := optionLabel(m.options[index], highlighted)
	fmt.Fprintf(b, "%s%s %s\n", marker, box, label)
}

func (m *multiSelectModel) countSelected() int {
	n := 0
	for _, on := range m.selected {
		if on {
			n++
		}
	}

	return n
}

// selectedOptions returns the selected Labels, in the order the options
// were listed in the config.
func (m *multiSelectModel) selectedOptions() []string {
	var selected []string
	for i, option := range m.options {
		if m.selected[i] {
			selected = append(selected, option.Label)
		}
	}

	return selected
}

// MultiSelect displays a multiple-choice terminal prompt and returns the
// chosen options, in the order they were listed in cfg.Options. It
// returns ErrAborted if the user cancels the prompt or if no options are
// provided, and a non-nil error if ctx is canceled while the prompt is
// running.
func MultiSelect(ctx context.Context, cfg MultiSelectConfig) ([]string, error) {
	if len(cfg.Options) == 0 {
		return nil, ErrAborted
	}

	finalModel, err := tea.NewProgram(newMultiSelectModel(cfg), tea.WithContext(ctx)).Run()
	if err != nil {
		return nil, fmt.Errorf("running multi-select prompt: %w", err)
	}

	m := finalModel.(*multiSelectModel) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.aborted {
		return nil, ErrAborted
	}

	return m.selectedOptions(), nil
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
// returns ErrAborted if the user cancels the prompt, and a non-nil error
// if ctx is canceled while the prompt is running.
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
// the page size survey used. A longer list would push the prompt itself
// out of view. The terminal height (tea.WindowSizeMsg) is not consulted:
// these prompts render inline rather than taking over the screen.
const maxVisibleOptions = 7

// noMatchesNote replaces the footer when nothing matches the filter,
// where the movement and selection hints would describe an empty list.
const noMatchesNote = "(no options match the filter, esc to clear it)"

// keyResult is what optionList.handleKey did with a key press.
type keyResult int

const (
	keyIgnored keyResult = iota
	keyHandled
	keyAborted
)

// optionList is the filtering, scrolling core shared by the list prompts.
// It owns the filter, which options match it, which of those are on
// screen, and every rendered line except the options. Enclosing models
// supply the meaning of Enter and how one option is drawn.
type optionList[T any] struct {
	message string
	help    string
	options []Option[T]

	// filter is rendered only once non-empty; until then the footer
	// advertises it.
	filter  textinput.Model
	initCmd tea.Cmd

	// matches holds the indexes into options that match filter, in their
	// original order, and every index while filter is empty. Indexing
	// into the full list rather than a narrowed-down copy keeps the
	// option acted on the one the cursor was on, and keeps a selection
	// valid across a change of filter.
	matches []int

	// cursor and offset are positions in matches: the highlighted option,
	// and the first option on screen.
	cursor int
	offset int

	// aborted records that the user abandoned the prompt.
	aborted bool
}

// newOptionList builds an optionList over options, with every option
// matching and the filter focused.
func newOptionList[T any](message, help string, options []Option[T]) optionList[T] {
	ti := textinput.New()
	ti.Prompt = "filter: "

	l := optionList[T]{message: message, help: help, options: options, filter: ti}
	l.initCmd = l.filter.Focus()
	l.applyFilter()

	return l
}

func (l *optionList[T]) Init() tea.Cmd {
	return l.initCmd
}

// handleKey handles the presses that work the list itself: moving the
// cursor, clearing the filter, and abandoning the prompt.
func (l *optionList[T]) handleKey(key string) keyResult {
	switch key {
	case keyCtrlC:
		l.aborted = true
		return keyAborted
	case keyEsc:
		// Esc clears a filter before it abandons the prompt, so that a
		// filter matching nothing can be undone.
		if l.filter.Value() != "" {
			l.filter.SetValue("")
			l.applyFilter()

			return keyHandled
		}

		l.aborted = true

		return keyAborted
	case "up", "ctrl+p":
		l.moveCursor(-1)
		return keyHandled
	case "down", "ctrl+n":
		l.moveCursor(1)
		return keyHandled
	}

	return keyIgnored
}

// updateFilter forwards msg to the filter field, re-narrowing the options
// if the filter text changed.
func (l *optionList[T]) updateFilter(msg tea.Msg) tea.Cmd {
	before := l.filter.Value()

	var cmd tea.Cmd
	l.filter, cmd = l.filter.Update(msg)
	if l.filter.Value() != before {
		l.applyFilter()
	}

	return cmd
}

// applyFilter re-narrows matches to the options whose Label contains the
// filter text, case-insensitively. The window returns to the top of the
// list, since the highlighted option may not have survived the change.
func (l *optionList[T]) applyFilter() {
	needle := strings.ToLower(l.filter.Value())

	l.matches = make([]int, 0, len(l.options))
	for i, option := range l.options {
		if needle == "" || strings.Contains(strings.ToLower(option.Label), needle) {
			l.matches = append(l.matches, i)
		}
	}

	l.cursor = 0
	l.offset = 0
}

// moveCursor moves the cursor delta options through the matching ones,
// clamped to the ends of the list, scrolling the window to keep the
// cursor inside it.
func (l *optionList[T]) moveCursor(delta int) {
	l.cursor = min(max(l.cursor+delta, 0), max(len(l.matches)-1, 0))

	switch {
	case l.cursor < l.offset:
		l.offset = l.cursor
	case l.cursor >= l.offset+maxVisibleOptions:
		l.offset = l.cursor - maxVisibleOptions + 1
	}
}

// highlighted returns the option the cursor is on and its index in
// options. ok is false while nothing matches the filter.
func (l *optionList[T]) highlighted() (option Option[T], index int, ok bool) {
	if len(l.matches) == 0 {
		return option, 0, false
	}

	index = l.matches[l.cursor]

	return l.options[index], index, true
}

// windowEnd returns the position in matches just past the last option on
// screen.
func (l *optionList[T]) windowEnd() int {
	return min(l.offset+maxVisibleOptions, len(l.matches))
}

// writeHeader renders the message, the help line, and the filter field
// once there is anything in it.
func (l *optionList[T]) writeHeader(b *strings.Builder) {
	fmt.Fprintln(b, messageStyle.Render(l.message))
	if l.help != "" {
		fmt.Fprintln(b, helpStyle.Render(l.help))
	}
	if l.filter.Value() != "" {
		fmt.Fprintln(b, l.filter.View())
	}
}

// writeOptions renders the options on screen, drawing each with row. row
// receives an option's index in options, so that it can reach whatever
// the enclosing model tracks per option.
func (l *optionList[T]) writeOptions(b *strings.Builder, row func(b *strings.Builder, index int, highlighted bool)) {
	// With nothing matching the filter, the footer stands in for the
	// options.
	_, highlighted, ok := l.highlighted()
	if !ok {
		return
	}

	for _, idx := range l.matches[l.offset:l.windowEnd()] {
		row(b, idx, idx == highlighted)
	}
}

// footer renders the line under the options: which part of the list is on
// screen, and what the keys do. hints are the enclosing model's own,
// listed last.
func (l *optionList[T]) footer(hints ...string) string {
	if len(l.matches) == 0 {
		return noMatchesNote
	}

	clauses := make([]string, 0, len(hints)+2)

	// The position replaces the movement hint rather than joining it: it
	// implies there is more list, and MultiSelect's footer passes 80
	// columns with both.
	if len(l.matches) > maxVisibleOptions {
		clauses = append(clauses, fmt.Sprintf("showing %d-%d of %d", l.offset+1, l.windowEnd(), len(l.matches)))
	} else {
		clauses = append(clauses, "up/down to move")
	}
	clauses = append(clauses, "type to filter")
	clauses = append(clauses, hints...)

	return "(" + strings.Join(clauses, ", ") + ")"
}

// optionLabel returns the marker drawn before an option and its label,
// highlighted under the cursor and dimmed if the option is disabled. A
// plain label skips lipgloss, whose Render costs microseconds per row to
// return the bytes it was given.
func optionLabel[T any](option Option[T], highlighted bool) (marker, label string) {
	switch {
	case highlighted:
		return "> ", selectedStyle.Render(option.Label)
	case option.Disabled:
		return "  ", helpStyle.Render(option.Label)
	default:
		return "  ", option.Label
	}
}

// stringOptions builds the options for a prompt whose choices are plain
// strings, each Value being the string itself.
func stringOptions(labels ...string) []Option[string] {
	options := make([]Option[string], len(labels))
	for i, label := range labels {
		options[i] = Option[string]{Label: label, Value: label}
	}

	return options
}

// listModel is the Bubble Tea model backing PickOne, and through it
// Select.
type listModel[T any] struct {
	optionList[T]

	// status explains why the last Enter press picked nothing. The next
	// key press clears it.
	status string

	// chosen is the picked option, or nil if the user abandoned the
	// prompt.
	chosen *Option[T]
}

// newListModel builds a listModel offering options under message.
func newListModel[T any](message, help string, options []Option[T]) *listModel[T] {
	return &listModel[T]{optionList: newOptionList(message, help, options)}
}

// Update handles the presses the list itself doesn't: Enter picks the
// highlighted option unless it is disabled. Anything else goes to the
// filter field.
func (m *listModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, m.updateFilter(msg)
	}

	// The status answered the previous key press.
	m.status = ""

	key := keyMsg.String()
	switch m.handleKey(key) {
	case keyAborted:
		return m, tea.Quit
	case keyHandled:
		return m, nil
	case keyIgnored:
	}

	if key == keyEnter {
		// With nothing matching, Enter has no option to return and waits
		// for the filter to be loosened.
		option, _, ok := m.highlighted()
		if !ok {
			return m, nil
		}

		if option.Disabled {
			m.status = cmp.Or(option.Description, "This option cannot be picked.")
			return m, nil
		}

		m.chosen = &option

		return m, tea.Quit
	}

	return m, m.updateFilter(msg)
}

func (m *listModel[T]) View() tea.View {
	var b strings.Builder

	m.writeHeader(&b)
	m.writeOptions(&b, m.writeOption)
	fmt.Fprintln(&b, helpStyle.Render(m.footer("enter to select")))
	if m.status != "" {
		fmt.Fprintln(&b, errorStyle.Render(m.status))
	}

	return tea.NewView(b.String())
}

// writeOption renders one option's line, plus a second line for a
// Description that annotates the option rather than explaining why it is
// disabled (see Option.Description).
func (m *listModel[T]) writeOption(b *strings.Builder, index int, highlighted bool) {
	option := m.options[index]

	marker, label := optionLabel(option, highlighted)
	fmt.Fprintf(b, "%s%s\n", marker, label)

	if option.Description != "" && !option.Disabled {
		fmt.Fprintf(b, "    %s\n", helpStyle.Render(option.Description))
	}
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
	// PickOne alone reports a cancel through ok rather than as
	// ErrAborted.
	chosen, err := pickOne(ctx, title, "", options)
	if errors.Is(err, ErrAborted) {
		return value, false, nil
	}
	if err != nil {
		return value, false, err
	}

	return chosen, true, nil
}

// pickOne is PickOne with the help line that Select's config exposes and
// PickOne's arguments have no room for, reporting a cancel as ErrAborted
// like the rest of this package.
func pickOne[T any](ctx context.Context, message, help string, options []Option[T]) (value T, err error) {
	finalModel, err := tea.NewProgram(newListModel(message, help, options), tea.WithContext(ctx)).Run()
	if err != nil {
		return value, fmt.Errorf("running picker: %w", err)
	}

	m := finalModel.(*listModel[T]) //nolint:forcetypeassert // Why: we control the only model given to this Program.
	if m.chosen == nil {
		return value, ErrAborted
	}

	return m.chosen.Value, nil
}
