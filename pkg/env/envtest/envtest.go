// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Package envtest provides test configuration overrides.
//
// It carries no build constraint, so test helpers that fake config compile
// under any build. Faking is opt-in at runtime rather than at build time: an
// override only takes effect once FakeTestConfig installs the override reader,
// so a binary that never calls it reads config exactly as it would in
// production.
package envtest

import (
	"fmt"
	"sync"

	"github.com/getoutreach/gobox/pkg/cfg"
	"go.yaml.in/yaml/v3"
)

type testOverrides struct {
	data map[string]interface{}
	mu   sync.Mutex
}

func (to *testOverrides) addWithError(k string, v interface{}) error {
	to.mu.Lock()
	defer to.mu.Unlock()

	if _, exists := to.data[k]; exists {
		return fmt.Errorf("repeated test override of '%s'", k)
	}

	to.data[k] = v
	return nil
}

func (to *testOverrides) load(k string) (interface{}, bool) {
	to.mu.Lock()
	defer to.mu.Unlock()

	// Apparently you cannot pull the bool out of this access implicitly in the return
	// statement.
	v, ok := to.data[k]

	return v, ok
}

func (to *testOverrides) delete(k string) {
	to.mu.Lock()
	defer to.mu.Unlock()

	delete(to.data, k)
}

// nolint:gochecknoglobals // Why: needs to be overridable
var overrides = testOverrides{
	data: make(map[string]interface{}),
}

// nolint:gochecknoglobals // Why: guards a one-time process-wide install
var installOnce sync.Once

// Install routes config reads through the override table, wrapping whatever
// reader is currently installed. It is idempotent and is called automatically
// by FakeTestConfig, so tests rarely need to call it directly.
func Install() {
	installOnce.Do(func() {
		cfg.SetDefaultReader(reader(cfg.DefaultReader()))
	})
}

// reader returns a cfg.Reader serving registered overrides, falling back to
// fallback for names with no override.
func reader(fallback cfg.Reader) cfg.Reader {
	return cfg.Reader(func(fileName string) ([]byte, error) {
		if override, ok := overrides.load(fileName); ok {
			return yaml.Marshal(override)
		}
		return fallback(fileName)
	})
}

// FakeTestConfig allows you to fake the test config with a specific value.
//
// The provided value is serialized to yaml and so can be structured data.
//
// Be extra careful when using this function in parallelized tests - do not
// use the fName across two tests running in parallel. This will cause the
// function to potentially panic.
//
// Please use `FakeTestConfigWithError` if you want an error returned rather than panicking
func FakeTestConfig(fName string, ptr interface{}) func() {
	// add ensures that it doesn't already exist to prevent two tests running
	// concurrently colliding on fName.
	f, err := FakeTestConfigWithError(fName, ptr)
	if err != nil {
		panic(fmt.Sprintf("failed to addHandler '%v'. Use the function 'FakeTestConfigWithError()' to capture the err message", err.Error()))
	}
	return f
}

// FakeTestConfigWithError allows you to fake the test config with a specific value
// and returns an error if a config with the same name exists already. If callers get an error,
// they should switch to running tests in serial.
func FakeTestConfigWithError(fName string, ptr interface{}) (func(), error) {
	Install()

	err := overrides.addWithError(fName, ptr)
	if err != nil {
		return nil, err
	}

	return func() {
		overrides.delete(fName)
	}, nil
}
