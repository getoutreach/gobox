//go:build or_e2e
// +build or_e2e

package env

import "github.com/getoutreach/gobox/pkg/cfg"

func ApplyOverrides() {
	old := cfg.DefaultReader()
	cfg.SetDefaultReader(testReader(devReader(old), testOverrides))
}

func init() { //nolint:gochecknoinits // Why: On purpose.
	ApplyOverrides()
}
