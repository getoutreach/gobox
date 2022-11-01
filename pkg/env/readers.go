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

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"gopkg.in/yaml.v3"
)

// nolint:gochecknoglobals
var testOverrides = make(map[string]interface{})

// linter is not aware of or_dev tags, so it falsely considers this deadcode.
func devReader(fallback cfg.Reader) cfg.Reader { //nolint:deadcode,unused
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

func testReader(fallback cfg.Reader, overrides map[string]interface{}) cfg.Reader {
	return cfg.Reader(func(fileName string) ([]byte, error) {
		if override, ok := overrides[fileName]; ok {
			return yaml.Marshal(override)
		}
		return fallback(fileName)
	})
}

// FakeTestConfig allows you to fake the test config with a specific value.
//
// The provided value is serialized to yaml and so can be structured data.
func FakeTestConfig(fName string, ptr interface{}) func() {
	if _, ok := testOverrides[fName]; ok {
		// This is not ideal.  We would prefer to return an error.
		// However, this function's signature does not support it and we
		// don't want to incur the backwards-incompatibility of changing
		// it right now.
		panic(fmt.Errorf("repeated test override of '%s'", fName))
	}

	testOverrides[fName] = ptr
	return func() {
		delete(testOverrides, fName)
	}
}
