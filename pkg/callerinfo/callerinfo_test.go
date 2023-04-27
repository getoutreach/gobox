package callerinfo

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func Test_ParsePackageName(t *testing.T) {
	assert.Equal(t,
		parsePackageName("github.com/getoutreach/gobox/pkg/foobar"),
		"github.com/getoutreach/gobox/pkg/foobar")
	assert.Equal(t,
		parsePackageName("github.com/getoutreach/gobox/pkg/callerinfo.Test_Callers"),
		"github.com/getoutreach/gobox/pkg/callerinfo")
	assert.Equal(t,
		parsePackageName("github.com/getoutreach/gobox/pkg/log.logger.Info"),
		"github.com/getoutreach/gobox/pkg/log")

	assert.Equal(t,
		parsePackageName("sdfdsfsed"),
		"error:sdfdsfsed")
}

func Test_Callers(t *testing.T) {
	assert.Equal(t, len(moduleLookupByPC), 0)

	ci, err := GetCallerInfo(0)
	assert.NilError(t, err)
	assert.Equal(t, ci.Package, "github.com/getoutreach/gobox/pkg/callerinfo")
	assert.Equal(t, ci.Function, "github.com/getoutreach/gobox/pkg/callerinfo.Test_Callers")
	assert.Check(t, strings.HasSuffix(ci.File, "callerinfo_test.go"))
	assert.Check(t, ci.LineNum > 0)
	assert.Equal(t, ci.Module, "github.com/getoutreach/gobox")
	// Until https://github.com/golang/go/issues/33976 is fixed, module info is not available in unit tests >_<
	// assert.Check(t, ci.ModuleVersion != "")

	assert.Equal(t, len(moduleLookupByPC), 1)

	ci2, err2 := testhelper1()
	assert.NilError(t, err2)
	assert.Equal(t, ci2.Function, "github.com/getoutreach/gobox/pkg/callerinfo.Test_Callers")

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
func testhelper1() (CallerInfo, error) {
	return GetCallerInfo(1)
}

//go:noinline
func testhelper2() (CallerInfo, error) {
	return GetCallerInfo(0)
}
