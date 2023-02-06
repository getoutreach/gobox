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
)

type testOverridesHandler struct {
	data map[string]interface{}
	mu   sync.Mutex
}

func (to *testOverridesHandler) addHandler(k string, v interface{}) error {
	to.mu.Lock()
	defer to.mu.Unlock()

	if _, exists := to.data[k]; exists {
		return fmt.Errorf("repeated test override of '%s'", k)
	}

	to.data[k] = v
	return nil
}

func (to *testOverridesHandler) deleteHandler(k string) {
	to.mu.Lock()
	defer to.mu.Unlock()

	delete(to.data, k)
}

// nolint:gochecknoglobals // Why: needs to be overridable
var overridesHandler = testOverridesHandler{
	data: make(map[string]interface{}),
}

// linter is not aware of or_dev tags, so it falsely considers this deadcode.
func devReaderHandler(fallback cfg.Reader) cfg.Reader { //nolint:deadcode,unused // Why: only used with certain build tags
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

// FakeTestConfigHandler allows you to fake the test config with a specific value.
func FakeTestConfigHandler(fName string, ptr interface{}) (func(), error) {
	err := overridesHandler.addHandler(fName, ptr)
	if err != nil {
		return nil, err
	}

	return func() {
		overridesHandler.deleteHandler(fName)
	}, nil
}
