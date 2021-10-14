//go:build !or_test && !or_dev && !or_e2e
// +build !or_test,!or_dev,!or_e2e

// env provides environment specific overrides
//
// All the functions provided by this package are meant to be called
// at app initialization and will effectively not do anything at all
// in production.
//
// This is done via build tags: or_test and or_dev represent the CI and
// dev-env environments.  The tags use the or_ prefix just in case
// some package in the dependency chain uses the same build tag to
// change their own behavior
package env

func ApplyOverrides() {
	// no overrides for non-dev & non-test environments
}
