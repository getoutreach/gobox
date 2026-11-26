// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for LookPathWith.

package exec_test

import (
	osexec "os/exec"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/exec"
	"github.com/getoutreach/gobox/pkg/exec/execstub"
)

// TestLookPathWith checks that an Exec that is a LookPather answers the
// lookup, and that any other Exec uses the real PATH.
func TestLookPathWith(t *testing.T) {
	t.Parallel()

	realSh, err := osexec.LookPath("sh")
	assert.NilError(t, err)

	t.Run("a LookPather answers", func(t *testing.T) {
		t.Parallel()

		s := execstub.NewStubs(t)
		s.RegisterLookPath("tool", "/fake/bin/tool")
		s.RegisterLookPathError("sh", &exec.Error{Name: "sh", Err: exec.ErrNotFound})

		got, err := exec.LookPathWith(s, "tool")
		assert.NilError(t, err)
		assert.Equal(t, got, "/fake/bin/tool")

		// The fake does not fall back to PATH, where sh exists.
		_, err = exec.LookPathWith(s, "sh")
		assert.ErrorIs(t, err, exec.ErrNotFound)
	})

	t.Run("an Exec that is not a LookPather uses PATH", func(t *testing.T) {
		t.Parallel()

		// A struct that embeds the Exec interface has only its methods.
		got, err := exec.LookPathWith(struct{ exec.Exec }{}, "sh")
		assert.NilError(t, err)
		assert.Equal(t, got, realSh)
	})
}
