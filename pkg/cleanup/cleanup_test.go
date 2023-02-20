//go:build !or_e2e

package cleanup_test

import (
	"fmt"
	"testing"

	"github.com/getoutreach/gobox/pkg/cleanup"
	"github.com/getoutreach/gobox/pkg/shuffler"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func ExampleFuncs() {
	var cleanup1, cleanup2, cleanup3 func()
	cleanups := cleanup.Funcs{&cleanup1, &cleanup2, &cleanup3}
	defer cleanups.Run()

	cleanup1 = func() { fmt.Println("first") }
	cleanup2 = func() { fmt.Println("second") }
	cleanup3 = func() { fmt.Println("third") }

	// you can return this.  once All is called, cleanups.Run does
	// nothing.
	all := cleanups.All()

	// calling all causes all the cleanup functions to be called.
	all()

	// Output: third
	// second
	// first
}

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestFuncsRun(t *testing.T) {
	entries := []string{}
	first := func() { entries = append(entries, "first") }
	second := func() { entries = append(entries, "second") }
	third := func() { entries = append(entries, "third") }

	// validate run ordering
	(&cleanup.Funcs{&first, &second, &third}).Run()
	assert.DeepEqual(t, entries, []string{"third", "second", "first"})

	// volidate that a panic of one does not cause previous cleanups to be missed
	first = func() { entries = nil }
	second = func() { panic("foo") }
	assert.Assert(t, cmp.Panics((&cleanup.Funcs{&first, &second}).Run))
	assert.Equal(t, len(entries), 0)

	// validate nil works
	var nilf func()
	(&cleanup.Funcs{&nilf}).Run()
}

func (suite) TestFuncsAll(t *testing.T) {
	entries := []string{}
	first := func() { entries = append(entries, "first") }
	second := func() { entries = append(entries, "second") }
	third := func() { entries = append(entries, "third") }

	all := func() func() {
		cleanups := cleanup.Funcs{&first, &second, &third}
		defer cleanups.Run()

		return cleanups.All()
	}()
	assert.Equal(t, len(entries), 0)

	all()
	assert.DeepEqual(t, entries, []string{"third", "second", "first"})
}
