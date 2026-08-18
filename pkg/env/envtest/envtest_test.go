// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Unit tests for the envtest config overrides

package envtest_test

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/env/envtest"
	"gotest.tools/v3/assert"
)

type listenConfig struct {
	ListenHost string `yaml:"ListenHost"`
	HTTPPort   int    `yaml:"HTTPPort"`
}

// TestFakeTestConfigServesOverrideWithoutBuildTags asserts that a faked config
// is readable through cfg.Load in a build with no or_* tags set. This file
// carries no build constraint on purpose: it is the regression guard for
// callers that cannot set build tags.
func TestFakeTestConfigServesOverrideWithoutBuildTags(t *testing.T) {
	cleanup := envtest.FakeTestConfig("envtest_serves.yaml", listenConfig{
		ListenHost: "someURL",
		HTTPPort:   8080,
	})
	defer cleanup()

	var got listenConfig
	assert.NilError(t, cfg.Load("envtest_serves.yaml", &got))
	assert.Equal(t, got.ListenHost, "someURL")
	assert.Equal(t, got.HTTPPort, 8080)
}

// TestCleanupRemovesOverride asserts the returned function unregisters the
// override, so a later read falls through to the underlying reader.
func TestCleanupRemovesOverride(t *testing.T) {
	cleanup := envtest.FakeTestConfig("envtest_cleanup.yaml", listenConfig{ListenHost: "gone"})

	var got listenConfig
	assert.NilError(t, cfg.Load("envtest_cleanup.yaml", &got))
	assert.Equal(t, got.ListenHost, "gone")

	cleanup()

	assert.Assert(t, cfg.Load("envtest_cleanup.yaml", &got) != nil)
}

// TestFakeTestConfigWithErrorRejectsRepeatedName asserts that faking the same
// name twice reports an error rather than silently clobbering the first value.
func TestFakeTestConfigWithErrorRejectsRepeatedName(t *testing.T) {
	cleanup, err := envtest.FakeTestConfigWithError("envtest_repeat.yaml", listenConfig{})
	assert.NilError(t, err)
	defer cleanup()

	_, err = envtest.FakeTestConfigWithError("envtest_repeat.yaml", listenConfig{})
	assert.Error(t, err, "repeated test override of 'envtest_repeat.yaml'")
}
