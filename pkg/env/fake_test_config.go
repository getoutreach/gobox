// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides configuration reader overriding optionss

package env

import (
	"fmt"
	"sync"
)

// nolint:gochecknoglobals // Why: needs to be overridable
var overrides = testOverrides{
	data: make(map[string]interface{}),
}

// testOverrides is a struct that allows us to override the test config
type testOverrides struct {
	data map[string]interface{}
	mu   sync.Mutex
}

// add adds a new key to the testOverrides map.
func (to *testOverrides) add(k string, v interface{}) {
	to.mu.Lock()
	defer to.mu.Unlock()

	if _, exists := to.data[k]; exists {
		// This is not ideal.  We would prefer to return an error. However
		// the caller function's signature does not support it and we don't
		// want to incur the backwards-incompatibility of changing it right
		// now.
		panic(fmt.Errorf("repeated test override of '%s'", k))
	}

	to.data[k] = v
}

// loadTestConfig loads the test config from the testOverrides map.
func (to *testOverrides) load(k string) (interface{}, bool) {
	to.mu.Lock()
	defer to.mu.Unlock()

	// Apparently you cannot pull the bool out of this access implicitly in the return
	// statement.
	v, ok := to.data[k]

	return v, ok
}

// delete removes a key from the testOverrides map.
func (to *testOverrides) delete(k string) {
	to.mu.Lock()
	defer to.mu.Unlock()

	delete(to.data, k)
}

// FakeTestConfig allows you to fake the test config with a specific value.
//
// The provided value is serialized to yaml and so can be structured data.
//
// Be extra careful when using this function in parallelized tests - do not
// use the fName across two tests running in parallel. This will cause the
// function to potentially panic.
//
// TODO[DT-3185]: Related work item to make the safety of this function better
func FakeTestConfig(fName string, ptr interface{}) func() {
	// add ensures that it doesn't already exist to prevent two tests running
	// concurrently colliding on fName.
	overrides.add(fName, ptr)

	return func() {
		overrides.delete(fName)
	}
}
