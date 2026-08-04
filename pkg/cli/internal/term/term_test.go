// Copyright 2026 Outreach Corporation. All Rights Reserved.

package term

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsTerminalFalseForRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "term-test")
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, f.Close()) })

	assert.Equal(t, IsTerminal(f), false)
}

func TestClearLine(t *testing.T) {
	assert.Equal(t, ClearLine, "\r\033[K")
}
