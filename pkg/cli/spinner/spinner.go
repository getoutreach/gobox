// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Implements a simple animated spinner for indicating
// progress during a blocking operation, built on Bubble Tea's spinner
// component.

// Package spinner implements a small animated spinner on top of
// charm.land/bubbles/v2/spinner, for use in place of
// github.com/briandowns/spinner.
package spinner

import (
	"fmt"
	"io"
	"os"
	"time"

	barspinner "charm.land/bubbles/v2/spinner"
	"golang.org/x/term"
)

// Spinner animates a labeled spinner on os.Stderr while a blocking
// operation runs elsewhere. Call Start to begin animating, then Stop once
// the operation finishes; Stop clears the spinner's line.
//
// If os.Stderr isn't a terminal, Start instead prints the description once
// as a plain line, since an animated spinner would be meaningless in
// redirected or logged output; Stop is then a no-op.
type Spinner struct {
	description string
	model       barspinner.Model
	isTerm      bool
	out         io.Writer

	stop chan struct{}
	done chan struct{}
}

// New returns a Spinner labeled with description, using the same frames
// and cadence as github.com/briandowns/spinner's CharSets[9].
func New(description string) *Spinner {
	return &Spinner{
		description: description,
		model:       barspinner.New(barspinner.WithSpinner(barspinner.Line)),
		isTerm:      term.IsTerminal(int(os.Stderr.Fd())),
		out:         os.Stderr,
	}
}

// Start begins animating the spinner in a background goroutine. It's a
// no-op if the spinner is already running.
func (s *Spinner) Start() {
	if !s.isTerm {
		fmt.Fprintln(s.out, s.description) //nolint:errcheck // Why: Best effort
		return
	}

	if s.stop != nil {
		return
	}

	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.run()
}

// Stop halts the animation and clears the spinner's line. It's safe to
// call even if Start hasn't been called, or has already been stopped.
func (s *Spinner) Stop() {
	if s.stop == nil {
		return
	}

	close(s.stop)
	<-s.done
	s.stop = nil
}

// run redraws the spinner on its own ticker, matching the frame rate of
// the underlying Bubble Tea spinner model, until Stop closes s.stop.
func (s *Spinner) run() {
	defer close(s.done)

	ticker := time.NewTicker(s.model.Spinner.FPS)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			fmt.Fprint(s.out, "\r\033[K") //nolint:errcheck // Why: Best effort
			return
		case <-ticker.C:
			s.model, _ = s.model.Update(s.model.Tick())
			fmt.Fprintf(s.out, "\r\033[K%s %s", s.model.View(), s.description) //nolint:errcheck // Why: Best effort
		}
	}
}
