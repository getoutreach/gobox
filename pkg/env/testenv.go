//go:build (or_test || or_int) && !or_e2e
// +build or_test or_int
// +build !or_e2e

package env

import (
	"github.com/getoutreach/gobox/pkg/env/envtest"
)

func ApplyOverrides() {
	envtest.Install()
}

func init() { //nolint: gochecknoinits
	ApplyOverrides()
}
