// Copyright 2026 Outreach Corporation. All Rights Reserved.

package spinner

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSpinnerNonTerminalPrintsDescriptionOnce(t *testing.T) {
	var buf bytes.Buffer
	s := New("Checking for updates...")
	s.out = &buf
	s.isTerm = false

	s.Start()
	s.Stop()

	if got := buf.String(); got != "Checking for updates...\n" {
		t.Errorf("got %q, want a single description line", got)
	}
}

func TestSpinnerStopWithoutStartIsSafe(t *testing.T) {
	s := New("test")
	s.Stop() // must not block or panic
}

// newTestSpinner returns a Spinner wired to a buffer with a fast frame
// rate, so animation tests don't need to wait for a real spinner cadence.
func newTestSpinner(description string) (*Spinner, *bytes.Buffer) {
	var buf bytes.Buffer
	s := New(description)
	s.out = &buf
	s.isTerm = true
	s.model.Spinner.FPS = time.Millisecond
	return s, &buf
}

func TestSpinnerTerminalAnimatesAndClearsOnStop(t *testing.T) {
	s, buf := newTestSpinner("Checking for updates...")

	s.Start()
	time.Sleep(10 * time.Millisecond) // let a few frames render
	s.Stop()

	got := buf.String()
	if !strings.Contains(got, "Checking for updates...") {
		t.Errorf("output %q does not contain the description", got)
	}
	if !strings.HasSuffix(got, "\r\033[K") {
		t.Errorf("output %q does not end with the line-clear sequence", got)
	}
}

func TestSpinnerDoubleStopIsSafe(t *testing.T) {
	s, _ := newTestSpinner("test")

	s.Start()
	s.Stop()
	s.Stop() // must not block or panic
}

func TestSpinnerDoubleStartIsSafe(t *testing.T) {
	s, _ := newTestSpinner("test")

	s.Start()
	s.Start() // must not spawn a second goroutine or panic
	s.Stop()
}
