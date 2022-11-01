// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Package secrets manages secrets config for outreach applications
//
// All secrets are assumed to be stored securely in the filesystem.
// This is compatible with the k8s approach of fetch and mounting
// secrets on separate volume/files. See
// https://kubernetes.io/docs/concepts/configuration/secret/#use-cases
//
// For the dev environment, call InitDevSecrets to initialize the
// secrets provider with a custom implementation
package secrets

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// nolint:gochecknoglobals
var devLookup func(ctx context.Context, key string) ([]byte, error)

// Make this public such that can be used by test cases too.
func TryMapWindowsKeys(filePath string) string {
	if runtime.GOOS != "windows" {
		return filePath
	}

	filePath = filepath.FromSlash(filePath)

	// if it's relative path, no prefixing drive letter
	if strings.HasPrefix(filePath, "\\") {
		filePath = "C:" + filePath
	}

	return filePath
}

// SetDevLookup sets the lookup bypass for dev environments
func SetDevLookup(lookup func(context.Context, string) ([]byte, error)) func(context.Context, string) ([]byte, error) {
	old := devLookup
	devLookup = lookup
	return old
}

// Config fetches the secret for the provided config file path.
//
// Use MustConfig if a config is required, particularly on app init.
func Config(ctx context.Context, filePath string) (string, error) {
	readFilePath := TryMapWindowsKeys(filePath)
	result, err := os.ReadFile(readFilePath)
	if err != nil && devLookup != nil {
		result, err = devLookup(ctx, filePath)
	}
	return string(result), err
}

// MustConfig is like Config except it panics on error.
//
// Use this if a config is read at startup.  Use the Config() function
// if the config is fetched on a per-request basis
func MustConfig(ctx context.Context, filePath string) string {
	result, err := Config(ctx, filePath)
	if err != nil {
		panic(err)
	}
	return result
}
