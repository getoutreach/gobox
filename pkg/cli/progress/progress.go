// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Implements a byte-progress meter for long running
// transfers, e.g. downloads and archive extraction, built on Bubble Tea's
// progress bar component.

// Package progress implements a small byte-progress meter on top of
// charm.land/bubbles/v2/progress, for use in place of
// github.com/schollz/progressbar/v3.
package progress

import (
	"fmt"
	"io"
	"os"
	"time"

	barprogress "charm.land/bubbles/v2/progress"
	"github.com/getoutreach/gobox/pkg/cli/internal/term"
)

const (
	// redrawInterval throttles how often an interactive progress bar is
	// redrawn, so Write doesn't flood the terminal with escape codes.
	redrawInterval = 65 * time.Millisecond

	// plainInterval throttles how often a status line is printed when
	// stderr isn't a terminal, so redirected or logged output doesn't get
	// a line per Write call.
	plainInterval = time.Second
)

// Bytes is an io.WriteCloser that renders a labeled progress bar to
// os.Stderr as bytes are written through it. Wrap it around a transfer with
// io.MultiWriter (for an io.Writer destination) or io.TeeReader (for an
// io.Reader source).
//
// If total is <= 0 (e.g. the size wasn't known ahead of time), Bytes falls
// back to reporting only the count of bytes transferred, since a
// percentage can't be computed.
//
// If os.Stderr isn't a terminal, Bytes prints an occasional plain text
// status line instead of redrawing a bar in place, so piped or logged
// output isn't filled with redraw escape codes.
type Bytes struct {
	description string
	total       int64
	totalStr    string
	isTerm      bool
	bar         barprogress.Model
	out         io.Writer
	now         func() time.Time
	termWidth   func() (width int, ok bool)

	written  int64
	start    time.Time
	lastDraw time.Time
	closed   bool
}

// NewBytes returns a Bytes progress meter for a transfer of total bytes
// (pass <= 0 if the size isn't known ahead of time), labeled with
// description.
func NewBytes(total int64, description string) *Bytes {
	termWidth := func() (int, bool) { return term.Width(os.Stderr) }
	return newBytes(total, description, os.Stderr, term.IsTerminal(os.Stderr), time.Now, termWidth)
}

// newBytes builds a Bytes with explicit out/isTerm/now/termWidth, so
// tests can substitute a buffer, a controllable clock, and a fixed
// terminal width without going through NewBytes' os.Stderr defaults.
func newBytes(
	total int64, description string, out io.Writer, isTerm bool, now func() time.Time, termWidth func() (int, bool),
) *Bytes {
	b := &Bytes{
		description: description,
		total:       total,
		totalStr:    formatBytes(total),
		isTerm:      isTerm,
		bar:         barprogress.New(barprogress.WithDefaultBlend()),
		out:         out,
		now:         now,
		termWidth:   termWidth,
	}
	b.start = b.now()
	return b
}

// Write implements io.Writer, recording len(p) bytes as transferred and
// redrawing the progress bar if enough time has passed since the last
// redraw. It never returns an error.
func (b *Bytes) Write(p []byte) (int, error) {
	b.written += int64(len(p))
	b.draw(false)
	return len(p), nil
}

// Close redraws the progress bar a final time to reflect the final byte
// count and prints a trailing newline. It's safe to call multiple times.
func (b *Bytes) Close() error {
	if b.closed {
		return nil
	}
	b.closed = true

	b.draw(true)
	fmt.Fprintln(b.out) //nolint:errcheck // Why: Best effort
	return nil
}

// draw redraws the current progress, subject to throttling, unless force
// is set.
func (b *Bytes) draw(force bool) {
	interval := redrawInterval
	if !b.isTerm {
		interval = plainInterval
	}

	now := b.now()
	if !force && now.Sub(b.lastDraw) < interval {
		return
	}
	b.lastDraw = now

	rate := int64(0)
	if elapsed := now.Sub(b.start).Seconds(); elapsed > 0 {
		rate = int64(float64(b.written) / elapsed)
	}

	var line string
	switch {
	case b.total > 0 && b.isTerm:
		// The styled bar is only worth rendering when it'll actually be
		// shown; skip it in the plain, non-terminal case below.
		suffix := fmt.Sprintf("  %s/%s  %s/s", formatBytes(b.written), b.totalStr, formatBytes(rate))

		// Re-check the terminal width on every redraw (not just once at
		// construction) and size the bar to fill the remaining space, so
		// the line keeps fitting the terminal across resizes the same
		// way github.com/schollz/progressbar's OptionFullWidth did.
		if width, ok := b.termWidth(); ok {
			b.bar.SetWidth(width - len(b.description) - 1 - len(suffix))
		}

		percent := min(1, float64(b.written)/float64(b.total))
		line = b.description + " " + b.bar.ViewAs(percent) + suffix
	case b.total > 0:
		line = fmt.Sprintf("%s %s/%s  %s/s", b.description, formatBytes(b.written), b.totalStr, formatBytes(rate))
	default:
		line = fmt.Sprintf("%s %s  %s/s", b.description, formatBytes(b.written), formatBytes(rate))
	}

	if b.isTerm {
		fmt.Fprint(b.out, term.ClearLine, line) //nolint:errcheck // Why: Best effort
	} else {
		fmt.Fprintln(b.out, line) //nolint:errcheck // Why: Best effort
	}
}

// formatBytes renders n bytes as a human-readable string using binary
// (1024-based) units, e.g. "1.5 MiB".
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
