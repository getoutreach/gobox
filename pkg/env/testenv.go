//go:build or_test && !or_e2e
// +build or_test,!or_e2e

package env

import (
	"github.com/getoutreach/gobox/pkg/cfg"
)

func ApplyOverrides() {
	old := cfg.DefaultReader()
	cfg.SetDefaultReader(testReader(old, &overrides))
}

func init() { //nolint: gochecknoinits
	ApplyOverrides()
}
