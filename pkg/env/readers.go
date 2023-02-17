// Copyright 2022 Outreach Corporation. All Rights Reserved.

//go:build or_dev || or_test || or_e2e
// +build or_dev or_test or_e2e

// Description: Provides configuration readers for various environments

package env

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"gopkg.in/yaml.v3"
)

type testOverrides struct {
	data map[string]interface{}
	mu   sync.Mutex
}

// func (to *testOverrides) add(k string, v interface{}) {
// 	to.mu.Lock()
// 	defer to.mu.Unlock()

// 	if _, exists := to.data[k]; exists {
// 		// This is not ideal.  We would prefer to return an error. However
// 		// the caller function's signature does not support it and we don't
// 		// want to incur the backwards-incompatibility of changing it right
// 		// now.
// 		panic(fmt.Errorf("repeated test override of '%s'", k))
// 	}

// 	to.data[k] = v
// }

func (to *testOverrides) addHandler(k string, v interface{}) error {
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

// devReader creates a config reader specific to the dev environment.
func devReader(fallback cfg.Reader) cfg.Reader { //nolint:deadcode,unused // Why: only used with certain build tags
	return cfg.Reader(func(fileName string) ([]byte, error) {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}

		info := app.Info()
		lookupPaths := []string{
			filepath.Join(u.HomeDir, ".outreach", info.Name, fileName),
			filepath.Join(u.HomeDir, ".outreach", fileName),
		}

		var b []byte
		errors := make([]error, 0)
		for _, p := range lookupPaths {
			var err error
			b, err = os.ReadFile(p)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			// we found a config, so exit
			// and remove all errors
			errors = nil
			break
		}
		if len(errors) != 0 {
			return fallback(fileName)
		}

		return b, nil
	})
}

func testReader(fallback cfg.Reader, overrider *testOverrides) cfg.Reader {
	return cfg.Reader(func(fileName string) ([]byte, error) {
		if override, ok := overrider.load(fileName); ok {
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
// Deprecated: Please use `FakeTestConfigHandler`
func FakeTestConfig(fName string, ptr interface{}) func() {
	// add ensures that it doesn't already exist to prevent two tests running
	// concurrently colliding on fName.
	err := overrides.addHandler(fName, ptr)
	if err != nil {
		panic(fmt.Sprintf("failed to addHandler '%v'. Please use the function 'FakeTestConfigHandler()' instead", err.Error()))
	}
	return func() {
		overrides.delete(fName)
	}
}

// FakeTestConfigHandler allows you to fake the test config with a specific value
// and returns an error if a config with the same name exists already. If callers get an error,
// they should switch to running tests in serial.
func FakeTestConfigHandler(fName string, ptr interface{}) (func(), error) {
	err := overrides.addHandler(fName, ptr)
	if err != nil {
		return nil, err
	}

	return func() {
		overrides.delete(fName)
	}, nil
}
