// Copyright 2026 Outreach Corporation. All Rights Reserved.

package progress

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{name: "zero", n: 0, want: "0 B"},
		{name: "under a KiB", n: 512, want: "512 B"},
		{name: "exactly a KiB", n: 1024, want: "1.0 KiB"},
		{name: "fractional MiB", n: 1024*1024*3 + 1024*512, want: "3.5 MiB"},
		{name: "GiB", n: 1024 * 1024 * 1024 * 2, want: "2.0 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, formatBytes(tt.n), tt.want)
		})
	}
}

// fakeClock returns a stepped time.Time on each call, advancing by step
// every time now() is invoked, so tests can control elapsed time
// deterministically.
func fakeClock(step time.Duration) func() time.Time {
	cur := time.Unix(0, 0)
	return func() time.Time {
		t := cur
		cur = cur.Add(step)
		return t
	}
}

// unknownWidth reports that the terminal width can't be determined,
// matching what term.Width returns for a non-terminal file.
func unknownWidth() (int, bool) { return 0, false }

// newTestBytes returns a Bytes wired to a buffer, for tests that don't
// need to control the reported terminal width.
func newTestBytes(t *testing.T, total int64, isTerm bool, now func() time.Time) (*Bytes, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	return newBytes(total, "test", &buf, isTerm, now, unknownWidth), &buf
}

// mustWrite writes p to w, failing the test immediately if it errors.
func mustWrite(t *testing.T, w io.Writer, p []byte) {
	t.Helper()

	_, err := w.Write(p)
	assert.NilError(t, err)
}

// mustClose closes c, failing the test immediately if it errors.
func mustClose(t *testing.T, c io.Closer) {
	t.Helper()

	assert.NilError(t, c.Close())
}

func TestBytesWriteThrottlesRedraws(t *testing.T) {
	// Each call to now() advances by 1ms, well under redrawInterval, so
	// only the first Write (which always draws, since lastDraw is zero)
	// should produce output.
	b, buf := newTestBytes(t, 100, true, fakeClock(time.Millisecond))

	for range 5 {
		mustWrite(t, b, []byte("x"))
	}

	assert.Equal(t, strings.Count(buf.String(), "\r"), 1)
}

func TestBytesCloseIsIdempotent(t *testing.T) {
	b, buf := newTestBytes(t, 100, true, fakeClock(time.Millisecond))

	mustWrite(t, b, []byte(strings.Repeat("x", 100)))

	mustClose(t, b)
	afterFirstClose := buf.String()

	mustClose(t, b)

	assert.Equal(t, buf.String(), afterFirstClose)
	assert.Assert(t, strings.HasSuffix(afterFirstClose, "\n"),
		"Close output %q does not end with a trailing newline", afterFirstClose)
}

func TestBytesUnknownTotal(t *testing.T) {
	// total <= 0 means no percentage can be computed; the rendered line
	// should still report the transferred count, without a bar.
	b, buf := newTestBytes(t, 0, false, fakeClock(2*time.Second))

	mustWrite(t, b, []byte(strings.Repeat("x", 2048)))

	assert.Assert(t, cmp.Contains(buf.String(), "test"))
	assert.Assert(t, cmp.Contains(buf.String(), "2.0 KiB"))
}

func TestBytesNonTerminalUsesPlainLines(t *testing.T) {
	b, buf := newTestBytes(t, 100, false, fakeClock(2*time.Second))

	mustWrite(t, b, []byte(strings.Repeat("x", 50)))

	assert.Assert(t, !strings.Contains(buf.String(), "\r"),
		"non-terminal output should not contain carriage returns: %q", buf.String())
	assert.Assert(t, strings.HasSuffix(buf.String(), "\n"),
		"non-terminal output should be newline-terminated: %q", buf.String())
}

func TestBytesKeepsDefaultBarWidthWhenTerminalWidthUnknown(t *testing.T) {
	const defaultBarWidth = 40 // charm.land/bubbles/v2/progress's unexported defaultWidth

	b, _ := newTestBytes(t, 100, true, fakeClock(time.Millisecond))

	mustWrite(t, b, []byte("x"))

	assert.Equal(t, b.bar.Width(), defaultBarWidth)
}

func TestBytesResizesBarToTerminalWidth(t *testing.T) {
	const defaultBarWidth = 40 // charm.land/bubbles/v2/progress's unexported defaultWidth

	var buf bytes.Buffer
	narrow := func() (int, bool) { return 20, true }
	b := newBytes(100, "dl", &buf, true, fakeClock(time.Millisecond), narrow)

	mustWrite(t, b, []byte(strings.Repeat("x", 10)))

	// A 20-column terminal can't fit "dl" plus a 40-char bar plus the
	// byte-count/rate suffix; the bar must shrink to fit, the way
	// schollz/progressbar's OptionFullWidth did across resizes.
	assert.Assert(t, b.bar.Width() < defaultBarWidth,
		"bar width = %d, want it shrunk below the unfitted default (%d) for a 20-column terminal", b.bar.Width(), defaultBarWidth)
}

var _ io.WriteCloser = (*Bytes)(nil)
