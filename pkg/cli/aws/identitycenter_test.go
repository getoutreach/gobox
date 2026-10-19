// Copyright 2026 Outreach Corporation. All Rights Reserved.

package aws

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/processcreds"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/getoutreach/gobox/pkg/box"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
)

// errNoToken is what the SDK reports when it reads the SSO token cache
// itself and the token is missing or expired.
var errNoToken = &ssocreds.InvalidTokenError{Err: errors.New("the SSO session has expired or is invalid")}

// errProcess is what the SDK reports when the credential_process that
// orc writes exits with a failure.
var errProcess = &processcreds.ProviderError{Err: errors.New("error in credential_process: exit status 1")}

// errNetwork is a failure that a login cannot fix.
var errNetwork = errors.New("dial tcp: lookup portal.sso.us-west-2.amazonaws.com: no such host")

func Test_ensureSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		// resolves is the result of each config load, in order.
		resolves []bool
		// resolveErr is what an unresolved load reports; errNoToken
		// when nil.
		resolveErr error
		loginErr   error
		loadErr    error
		wantLogin  bool
		wantErr    bool
	}{
		{name: "valid session", resolves: []bool{true}, wantLogin: false},
		{name: "expired session", resolves: []bool{false, true}, wantLogin: true},
		{name: "still expired after login", resolves: []bool{false, false}, wantLogin: true, wantErr: true},
		{name: "login fails", resolves: []bool{false}, loginErr: errNoToken, wantLogin: true, wantErr: true},
		{name: "config unreadable", loadErr: errNoToken, wantLogin: false, wantErr: true},
		{
			name: "credential process fails", resolves: []bool{false, true}, resolveErr: errProcess,
			wantLogin: true,
		},
		{
			name: "network error", resolves: []bool{false}, resolveErr: errNetwork,
			wantLogin: false, wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var loads, logins int

			load := func() (aws.Config, error) {
				if tc.loadErr != nil {
					return aws.Config{}, tc.loadErr
				}

				loads++
				resolves := tc.resolves[loads-1]

				return aws.Config{Credentials: aws.CredentialsProviderFunc(
					func(context.Context) (aws.Credentials, error) {
						if !resolves {
							if tc.resolveErr != nil {
								return aws.Credentials{}, tc.resolveErr
							}

							return aws.Credentials{}, errNoToken
						}

						return aws.Credentials{AccessKeyID: "AKIATEST"}, nil
					},
				)}, nil
			}

			login := func() error {
				logins++
				return tc.loginErr
			}

			log := logrus.New()
			log.SetOutput(io.Discard)

			_, err := ensureSession(t.Context(), log, load, login)

			assert.Equal(t, err != nil, tc.wantErr, "expected err=%t, got: %v", tc.wantErr, err)
			assert.Equal(t, logins > 0, tc.wantLogin, "expected login=%t, got %d attempts", tc.wantLogin, logins)
		})
	}
}

// Test_staleProfileOverride covers the AWS_PROFILE a shell config exports
// after `orc setup` removed the profile it names.
func Test_staleProfileOverride(t *testing.T) {
	const config = "[profile mine]\nregion = us-west-2\n"

	for _, tc := range []struct {
		name string
		// env is set for the test, on top of the config file paths.
		env map[string]string
		// credentials is written to the credentials file when set.
		credentials string
		want        bool
		// wantWarning is the variable the warning names, when want is set.
		wantWarning string
	}{
		{name: "no profile", env: nil},
		{name: "profile in the config", env: map[string]string{"AWS_PROFILE": "mine"}},
		{name: "profile that is gone", env: map[string]string{"AWS_PROFILE": "readwrite"}, want: true, wantWarning: "AWS_PROFILE"},
		{
			name:        "profile in the credentials file",
			env:         map[string]string{"AWS_PROFILE": "static"},
			credentials: "[static]\naws_access_key_id = AKIATEST\naws_secret_access_key = secret\n",
		},
		{name: "AWS_DEFAULT_PROFILE in the config", env: map[string]string{"AWS_DEFAULT_PROFILE": "mine"}},
		{
			name:        "AWS_DEFAULT_PROFILE that is gone",
			env:         map[string]string{"AWS_DEFAULT_PROFILE": "readwrite"},
			want:        true,
			wantWarning: "AWS_DEFAULT_PROFILE",
		},
		{
			name:        "both set and gone",
			env:         map[string]string{"AWS_PROFILE": "readwrite", "AWS_DEFAULT_PROFILE": "readwrite"},
			want:        true,
			wantWarning: "AWS_PROFILE",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			cfgPath := filepath.Join(dir, "config")
			assert.NilError(t, os.WriteFile(cfgPath, []byte(config), 0o600),
				"expected to be able to write the config")
			t.Setenv("AWS_CONFIG_FILE", cfgPath)

			credsPath := filepath.Join(dir, "credentials")
			assert.NilError(t, os.WriteFile(credsPath, []byte(tc.credentials), 0o600),
				"expected to be able to write the credentials")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)

			// t.Setenv restores whatever the environment already held,
			// including a value this test did not set.
			t.Setenv("AWS_PROFILE", "")
			t.Setenv("AWS_DEFAULT_PROFILE", "")
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			log, hook := logtest.NewNullLogger()

			assert.Equal(t, staleProfileOverride(t.Context(), log), tc.want)

			if tc.wantWarning != "" {
				entry := hook.LastEntry()
				assert.Assert(t, entry != nil, "expected a warning")
				assert.Assert(t, strings.HasPrefix(entry.Message, tc.wantWarning+" names"),
					"expected the warning to name %s, got: %s", tc.wantWarning, entry.Message)
			}

			// The process environment is left alone.
			for _, key := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE"} {
				assert.Equal(t, os.Getenv(key), tc.env[key], "expected %s to be untouched", key)
			}
		})
	}
}

func Test_grantedLoginArgs(t *testing.T) {
	ic := &box.IdentityCenterConfig{StartURL: "https://example.awsapps.com/start", Region: "us-west-2"}

	assert.DeepEqual(t, grantedLoginArgs(ic), []string{
		"sso", "login",
		"--sso-region", "us-west-2",
		"--sso-start-url", "https://example.awsapps.com/start",
	})
}

func Test_grantedLogin_requiresConfig(t *testing.T) {
	for _, ic := range []*box.IdentityCenterConfig{
		{},
		{StartURL: "https://example.awsapps.com/start"},
		{Region: "us-west-2"},
	} {
		err := grantedLogin(t.Context(), ic)
		assert.ErrorContains(t, err, "needs startURL and region")
	}
}

func Test_refreshCredsViaIdentityCenter(t *testing.T) {
	b := &box.Config{AWS: box.AWSConfig{
		RefreshMethod: RefreshMethodIdentityCenter,
		IdentityCenter: box.IdentityCenterConfig{
			StartURL: "https://example.awsapps.com/start",
			Region:   "us-west-2",
		},
	}}

	t.Run("dry run prints the granted command", func(t *testing.T) {
		log, hook := logtest.NewNullLogger()
		copts := &CredentialOptions{Log: log}

		err := refreshCredsViaIdentityCenter(t.Context(), copts, &AuthorizeCredentialsOptions{DryRun: true}, b)
		assert.NilError(t, err)

		entries := hook.AllEntries()
		assert.Equal(t, len(entries), 1)
		assert.Equal(t, entries[0].Message,
			"Dry Run: granted sso login --sso-region us-west-2 --sso-start-url https://example.awsapps.com/start")
	})

	t.Run("dry run with an empty config fails", func(t *testing.T) {
		log, _ := logtest.NewNullLogger()
		copts := &CredentialOptions{Log: log}

		err := refreshCredsViaIdentityCenter(t.Context(), copts, &AuthorizeCredentialsOptions{DryRun: true}, &box.Config{})
		assert.ErrorContains(t, err, "needs startURL and region")
	})

	t.Run("nil options and nil logger do not panic", func(t *testing.T) {
		for _, copts := range []*CredentialOptions{nil, {}} {
			err := refreshCredsViaIdentityCenter(t.Context(), copts, &AuthorizeCredentialsOptions{DryRun: true}, b)
			assert.NilError(t, err)
		}
	})

	t.Run("credential provider output is not supported", func(t *testing.T) {
		log, _ := logtest.NewNullLogger()
		copts := &CredentialOptions{Log: log}

		err := refreshCredsViaIdentityCenter(t.Context(), copts,
			&AuthorizeCredentialsOptions{Output: OutputCredentialProvider}, b)
		assert.ErrorContains(t, err, "does not support credential provider output")
	})
}

// writeBoxConfig writes a box config with the given aws section to a
// temporary HOME.
func writeBoxConfig(t *testing.T, aws string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, box.BoxConfigPath)
	assert.NilError(t, os.MkdirAll(dir, 0o755), "expected to be able to create the box config dir")

	contents := "version: 1\nconfig:\n  aws:\n" + aws
	assert.NilError(t, os.WriteFile(filepath.Join(dir, box.BoxConfigFile), []byte(contents), 0o600),
		"expected to be able to write the box config")
}

// Test_AuthorizeCredentials_identityCenter covers the dispatch to the
// identity-center refresh method.
func Test_AuthorizeCredentials_identityCenter(t *testing.T) {
	// The AWS_PROFILE and AWS_ROLE overrides of the box config would
	// otherwise depend on the environment of the test run.
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ROLE", "")

	identityCenterBox := "    refreshMethod: identity-center\n    identityCenter: {}\n"

	t.Run("dry run reaches the identity-center check", func(t *testing.T) {
		writeBoxConfig(t, identityCenterBox)

		log, _ := logtest.NewNullLogger()
		copts := &CredentialOptions{
			Log:      log,
			Profile:  "default",
			FileName: filepath.Join(t.TempDir(), "credentials"),
		}

		err := AuthorizeCredentials(t.Context(), copts, &AuthorizeCredentialsOptions{DryRun: true})
		assert.ErrorContains(t, err, "needs startURL and region")
	})

	t.Run("credential provider output is not supported", func(t *testing.T) {
		writeBoxConfig(t, identityCenterBox)

		log, _ := logtest.NewNullLogger()
		copts := &CredentialOptions{
			Log:      log,
			Profile:  "default",
			FileName: filepath.Join(t.TempDir(), "credentials"),
		}

		err := AuthorizeCredentials(t.Context(), copts, &AuthorizeCredentialsOptions{Output: OutputCredentialProvider})
		assert.ErrorContains(t, err, "does not support credential provider output")
	})

	t.Run("valid credentials and no box config is not an error", func(t *testing.T) {
		// A HOME without a box config makes box.LoadBox fail.
		t.Setenv("HOME", t.TempDir())

		credsPath := filepath.Join(t.TempDir(), "credentials")
		contents := "[default]\nx_security_token_expires = " + time.Now().Add(time.Hour).Format(time.RFC3339)
		assert.NilError(t, os.WriteFile(credsPath, []byte(contents), 0o600),
			"expected to be able to write the credentials")

		log, _ := logtest.NewNullLogger()
		copts := &CredentialOptions{Log: log, Profile: "default", FileName: credsPath}

		assert.NilError(t, AuthorizeCredentials(t.Context(), copts, &AuthorizeCredentialsOptions{}))
	})

	t.Run("valid credentials still reach identity-center", func(t *testing.T) {
		writeBoxConfig(t, identityCenterBox)

		credsPath := filepath.Join(t.TempDir(), "credentials")
		contents := "[default]\nx_security_token_expires = " + time.Now().Add(time.Hour).Format(time.RFC3339)
		assert.NilError(t, os.WriteFile(credsPath, []byte(contents), 0o600),
			"expected to be able to write the credentials")

		log, _ := logtest.NewNullLogger()
		copts := &CredentialOptions{Log: log, Profile: "default", FileName: credsPath}

		err := AuthorizeCredentials(t.Context(), copts, &AuthorizeCredentialsOptions{DryRun: true})
		assert.ErrorContains(t, err, "needs startURL and region")
	})
}
