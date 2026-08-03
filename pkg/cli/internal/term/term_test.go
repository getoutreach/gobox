// Copyright 2026 Outreach Corporation. All Rights Reserved.

package term

import (
	"os"
	"testing"
)

func TestIsTerminalFalseForRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "term-test")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	t.Cleanup(func() { f.Close() }) //nolint:errcheck // Why: Best effort

	if IsTerminal(f) {
		t.Error("IsTerminal(regular file) = true, want false")
	}
}

func TestClearLine(t *testing.T) {
	if want := "\r\033[K"; ClearLine != want {
		t.Errorf("ClearLine = %q, want %q", ClearLine, want)
	}
}
