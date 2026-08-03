// Copyright 2026 Outreach Corporation. All Rights Reserved.

package progress

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
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
			if got := formatBytes(tt.n); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
			}
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

func newTestBytes(total int64, isTerm bool, now func() time.Time) (*Bytes, *bytes.Buffer) {
	var buf bytes.Buffer
	return newBytes(total, "test", &buf, isTerm, now), &buf
}

func TestBytesWriteThrottlesRedraws(t *testing.T) {
	// Each call to now() advances by 1ms, well under redrawInterval, so
	// only the first Write (which always draws, since lastDraw is zero)
	// should produce output.
	b, buf := newTestBytes(100, true, fakeClock(time.Millisecond))

	for range 5 {
		if _, err := b.Write([]byte("x")); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if n := strings.Count(buf.String(), "\r"); n != 1 {
		t.Errorf("got %d redraws, want 1 (throttled)", n)
	}
}

func TestBytesCloseIsIdempotent(t *testing.T) {
	b, buf := newTestBytes(100, true, fakeClock(time.Millisecond))

	if _, err := b.Write([]byte(strings.Repeat("x", 100))); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	afterFirstClose := buf.String()

	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	if buf.String() != afterFirstClose {
		t.Errorf("second Close produced more output: got %q, want %q", buf.String(), afterFirstClose)
	}
	if !strings.HasSuffix(afterFirstClose, "\n") {
		t.Errorf("Close output %q does not end with a trailing newline", afterFirstClose)
	}
}

func TestBytesUnknownTotal(t *testing.T) {
	// total <= 0 means no percentage can be computed; the rendered line
	// should still report the transferred count, without a bar.
	b, buf := newTestBytes(0, false, fakeClock(2*time.Second))

	if _, err := b.Write([]byte(strings.Repeat("x", 2048))); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "test") || !strings.Contains(got, "2.0 KiB") {
		t.Errorf("unexpected output for unknown total: %q", got)
	}
}

func TestBytesNonTerminalUsesPlainLines(t *testing.T) {
	b, buf := newTestBytes(100, false, fakeClock(2*time.Second))

	if _, err := b.Write([]byte(strings.Repeat("x", 50))); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if strings.Contains(buf.String(), "\r") {
		t.Errorf("non-terminal output should not contain carriage returns: %q", buf.String())
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("non-terminal output should be newline-terminated: %q", buf.String())
	}
}

var _ io.WriteCloser = (*Bytes)(nil)
