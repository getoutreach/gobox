// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Compatibility wrappers for the envtest config overrides

package env

import (
	"github.com/getoutreach/gobox/pkg/env/envtest"
)

// FakeTestConfig fakes the config named fName with ptr, returning a function
// that removes the override. New code should call envtest.FakeTestConfig.
func FakeTestConfig(fName string, ptr interface{}) func() {
	return envtest.FakeTestConfig(fName, ptr)
}

// FakeTestConfigWithError fakes the config named fName with ptr, returning a
// function that removes the override, or an error if fName is already faked.
// New code should call envtest.FakeTestConfigWithError.
func FakeTestConfigWithError(fName string, ptr interface{}) (func(), error) {
	return envtest.FakeTestConfigWithError(fName, ptr)
}
