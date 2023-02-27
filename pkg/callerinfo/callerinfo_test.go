package callerinfo

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_Callers(t *testing.T) {
	assert.Equal(t, len(moduleLookupByPC), 0)

	caller, err := GetCallerFunction(0)
	assert.NilError(t, err)
	assert.Equal(t, caller, "github.com/getoutreach/gobox/pkg/callerinfo.Test_Callers")

	assert.Equal(t, len(moduleLookupByPC), 1)

	caller2, err2 := testhelper1()
	assert.NilError(t, err2)
	assert.Equal(t, caller2, "github.com/getoutreach/gobox/pkg/callerinfo.Test_Callers")

	// Same result, but different call site, so will be a new PC->Function lookup
	assert.Equal(t, len(moduleLookupByPC), 2)

	// This call will not be cached
	_, _ = testhelper2()
	assert.Equal(t, len(moduleLookupByPC), 3)
	// This call will be cached
	_, _ = testhelper2()
	assert.Equal(t, len(moduleLookupByPC), 3)
}

//go:noinline
func testhelper1() (string, error) {
	return GetCallerFunction(1)
}

//go:noinline
func testhelper2() (string, error) {
	return GetCallerFunction(0)
}
