// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import tea "charm.land/bubbletea/v2"

// keypress builds the key press for a printable character, such as a
// letter typed into a text field or matched against a single-character
// keybinding like "y".
func keypress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: string(r), Code: r, ShiftedCode: r})
}

// codeKeypress builds the key press for a named key, such as tea.KeyEnter
// or tea.KeyLeft, that has no printable text of its own.
func codeKeypress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

// ctrlKeypress builds the key press for a Ctrl+<r> combination, such as
// the ctrl+n and ctrl+p cursor movements.
func ctrlKeypress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: r, Mod: tea.ModCtrl})
}

// ctrlCKeypress builds the key press for Ctrl+C.
func ctrlCKeypress() tea.KeyPressMsg {
	return ctrlKeypress('c')
}

// typeString drives m.Update with a keypress for each rune in s, in order.
func typeString(m tea.Model, s string) {
	for _, r := range s {
		m.Update(keypress(r))
	}
}
