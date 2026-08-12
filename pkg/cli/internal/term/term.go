// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Shared terminal-output helpers for pkg/cli's Bubble Tea
// based line writers (progress, spinner).

// Package term holds small terminal-output helpers shared by pkg/cli's
// progress and spinner packages.
package term

import (
	"os"

	"golang.org/x/term"
)

// ClearLine is the ANSI escape sequence that returns the cursor to the
// start of the current line and erases it, used to redraw a status line
// in place.
const ClearLine = "\r\033[K"

// IsTerminal reports whether f is connected to a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Width returns f's current terminal column width, and whether it could
// be determined at all (e.g. f might not be a terminal). Callers that
// redraw a line in place should re-check this on every redraw rather
// than caching it, so output keeps fitting the terminal across resizes.
func Width(f *os.File) (width int, ok bool) {
	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil || w <= 0 {
		return 0, false
	}
	return w, true
}
