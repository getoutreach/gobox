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
