package secretstest

import (
	"context"
	"fmt"
	"os"

	"github.com/getoutreach/gobox/pkg/secrets"
)

// nolint:gochecknoglobals
var testOverrides map[string]string

// SetTestOverride overrides the lookup of a specific secret.
//
// Returns a function that undoes the override.
func setTestOverride(filePath, value string) (func(), error) {
	if testOverrides == nil {
		testOverrides = make(map[string]string)
	}
	key := secrets.TryMapWindowsKeys(filePath)
	if _, ok := testOverrides[key]; ok {
		return nil, fmt.Errorf("repeated test override of '%s'", filePath)
	}
	testOverrides[key] = value
	cleanup := func() { delete(testOverrides, key) }
	return cleanup, nil
}

// TestLookup is a lookup function that can be provided as an override
// to the usual `secrets` lookup implementation.
//
// TestLookup uses the global `testOverrides` declared elsewhere in this
// file to provide specific overrides for specific values.
func testLookup(ctx context.Context, filePath string) ([]byte, error) {
	key := secrets.TryMapWindowsKeys(filePath)
	if value, ok := testOverrides[key]; ok {
		return []byte(value), nil
	}
	return nil, os.ErrNotExist
}

// Mock updates secrets so that any fetch of key would return
// the provided value.  It returns the cleanup function.
//
// Usage:
//
//      func TestXYZ(t *testing.T) {
//           defer secretstest.Fake("/etc/.honeycomb_api_key", "SOME KEY")()
//           ... regular tests ..
//      }
func Fake(key, value string) func() {
	cleanup, err := setTestOverride(key, value)
	if err != nil {
		// This is not ideal.  We would prefer to return an error.
		// However, this function's signature does not support it and we
		// don't want to incur the backwards-incompatibility of changing
		// it right now.
		panic(err)
	}

	// Overrides the secrets `devLookup` function with our own.  It's
	// possible for this to happen multiple times, in which case we would
	// override `testLookup` with `testLookup`.  That's OK.  So long as the
	// `cleanup` functions are called in reverse order to how they were
	// returned, everything should work out fine.

	oldDevLookup := secrets.SetDevLookup(testLookup)
	return func() { cleanup(); secrets.SetDevLookup(oldDevLookup) }
}
