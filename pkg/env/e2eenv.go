// Copyright 2022 Outreach Corporation. All Rights Reserved.

//go:build or_e2e
// +build or_e2e

// Description: Provides environment overrides for e2e tests

package env

import (
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/env/envtest"
)

func ApplyOverrides() {
	cfg.SetDefaultReader(devReader(cfg.DefaultReader()))
	envtest.Install()
}

func init() { //nolint:gochecknoinits // Why: On purpose.
	ApplyOverrides()
}
