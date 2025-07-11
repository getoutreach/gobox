// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements consistent ways to get Auth across
// platforms.
package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// githubKey is the standard host used for github.com
// in the gh config
const githubKey = "github.com"

// ghPath is a memoized path to the `gh` CLI.
var ghPath = ""

// authAccessor is a function that returns a github token if
// available via this method.
type authAccessor func() (cfg.SecretData, error)

// GetToken returns a github access token from the machine
func GetToken() (cfg.SecretData, error) {
	accessors := []authAccessor{
		envToken,
		outreachDirToken,
		ghCLIAuthToken,

		// Deprecated: Use ghCLIAuthToken in the future. Leaving to ensure that
		// potentially non standard formats of the gh config are still supported.
		ghCLIToken,
	}

	errs := make([]error, 0)
	for _, accessor := range accessors {
		token, err := accessor()
		if err == nil {
			return token, nil
		}

		errs = append(errs, err)
	}

	return "", fmt.Errorf("failed to find github token: %v", errs)
}

// outreachDirToken reads a token from the legacy ~/.outreach/github.token
// path
func outreachDirToken() (cfg.SecretData, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "failed to get user home directory")
	}

	contents, err := os.ReadFile(filepath.Join(homeDir, ".outreach", "github.token"))
	if err != nil {
		return "", errors.Wrap(err, "failed to read token from ~/.outreach/github.token")
	}

	return cfg.SecretData(strings.TrimSpace(string(contents))), nil
}

// envToken reads a Github token from GITHUB_TOKEN or
// OUTREACH_GITHUB_TOKEN
func envToken() (cfg.SecretData, error) {
	envVars := []string{"GITHUB_TOKEN", "OUTREACH_GITHUB_TOKEN"}
	for _, envVar := range envVars {
		if v := os.Getenv(envVar); v != "" {
			return cfg.SecretData(v), nil
		}
	}

	return "", fmt.Errorf("failed to read token from env vars: %v", envVars)
}

// ghCLIAuthToken is a helper function to get the token from the gh CLI
// using the 'gh auth token' command.
func ghCLIAuthToken() (cfg.SecretData, error) {
	cmd, err := ghCmd("auth", "token")
	if err != nil {
		return "", err
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get token via gh CLI (try 'gh auth login?'): %s", string(b))
	}

	// Return the token, removing whitespace
	return cfg.SecretData(strings.TrimSpace(string(b))), nil
}

// ghCLIToken gets a token from gh, or informs the user how to setup
// a github token via gh, or install gh if not found
func ghCLIToken() (cfg.SecretData, error) {
	// Mostly just for tests that fake the value
	if os.Getenv("GOBOX_SKIP_VALIDATE_AUTH") != "true" {
		cmd, err := ghCmd("auth", "status")
		if err != nil {
			return "", err
		}
		if _, err := cmd.CombinedOutput(); err != nil {
			cmd, err := ghCmd("auth", "login")
			if err != nil {
				return "", err
			}
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				return "", errors.Wrap(err, "failed to login via gh CLI")
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "failed to get user home directory")
	}

	ghAuthPath := filepath.Join(homeDir, ".config", "gh", "hosts.yml")
	f, err := os.Open(ghAuthPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read gh auth configuration, try running 'gh auth login'")
	}
	defer f.Close()

	var conf map[string]interface{}
	err = yaml.NewDecoder(f).Decode(&conf)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse gh auth configuration")
	}

	if _, ok := conf[githubKey]; !ok {
		return "", fmt.Errorf("failed to find host '%s' in gh auth config, try running 'gh auth login'", githubKey)
	}

	realConf, ok := conf[githubKey].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("expected map[string]interface{} for %s host, got %v", githubKey, reflect.ValueOf(conf[githubKey]).String())
	}

	tokenInf, ok := realConf["oauth_token"]
	if !ok {
		return "", fmt.Errorf("failed to find oauth_token in gh auth config, try running 'gh auth login'")
	}

	token, ok := tokenInf.(string)
	if !ok {
		return "", fmt.Errorf("expected string for oauth_token, got %s", reflect.ValueOf(token).String())
	}

	return cfg.SecretData(token), nil
}

// ghCmd is a helper function to create a `exec.Cmd` for the `gh` CLI.
// If `mise` exists, it will grab the `gh` path from `mise which`.
func ghCmd(args ...string) (*exec.Cmd, error) {
	var err error
	if ghPath == "" {
		misePath, err := exec.LookPath("mise")
		if err == nil && misePath != "" {
			cmd := exec.Command(misePath, "which", "gh")
			whichPath, err := cmd.CombinedOutput()
			if err != nil {
				return nil, errors.Wrap(err, "failed to find 'gh' CLI via 'mise which gh'")
			}
			ghPath = strings.TrimSpace(string(whichPath))
		}
	}
	if ghPath == "" {
		ghPath, err = exec.LookPath("gh")
	}
	if err != nil || ghPath == "" {
		return nil, errors.New("failed to find 'gh' CLI")
	}
	return exec.Command(ghPath, args...), nil
}
