// Copyright 2026 Outreach Corporation. All Rights Reserved.

package aws

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/getoutreach/gobox/pkg/box"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
)

// errNoToken is what the SDK reports when the SSO token is missing or
// expired.
var errNoToken = errors.New("the SSO session has expired or is invalid")

func Test_ensureSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		// resolves is the result of each config load, in order.
		resolves  []bool
		loginErr  error
		loadErr   error
		wantLogin bool
		wantErr   bool
	}{
		{name: "valid session", resolves: []bool{true}, wantLogin: false},
		{name: "expired session", resolves: []bool{false, true}, wantLogin: true},
		{name: "still expired after login", resolves: []bool{false, false}, wantLogin: true, wantErr: true},
		{name: "login fails", resolves: []bool{false}, loginErr: errNoToken, wantLogin: true, wantErr: true},
		{name: "config unreadable", loadErr: errNoToken, wantLogin: false, wantErr: true},
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

// Test_clearStaleProfile covers the AWS_PROFILE a shell config exports
// after `orc setup` removed the profile it names.
func Test_clearStaleProfile(t *testing.T) {
	const config = "[profile mine]\nregion = us-west-2\n"

	for _, tc := range []struct {
		name string
		// env is set for the test, on top of the config file paths.
		env map[string]string
		// credentials is written to the credentials file when set.
		credentials string
		// want and wantDefault are the AWS_PROFILE and the
		// AWS_DEFAULT_PROFILE the call leaves behind.
		want        string
		wantDefault string
	}{
		{name: "no profile", env: nil},
		{name: "profile in the config", env: map[string]string{"AWS_PROFILE": "mine"}, want: "mine"},
		{name: "profile that is gone", env: map[string]string{"AWS_PROFILE": "readwrite"}},
		{
			name:        "profile in the credentials file",
			env:         map[string]string{"AWS_PROFILE": "static"},
			credentials: "[static]\naws_access_key_id = AKIATEST\naws_secret_access_key = secret\n",
			want:        "static",
		},
		{
			name:        "AWS_DEFAULT_PROFILE in the config",
			env:         map[string]string{"AWS_DEFAULT_PROFILE": "mine"},
			wantDefault: "mine",
		},
		{
			name: "AWS_DEFAULT_PROFILE that is gone",
			env:  map[string]string{"AWS_DEFAULT_PROFILE": "readwrite"},
		},
		{
			// AWS_PROFILE wins, so a stale one takes the other with it.
			name: "both set and gone",
			env:  map[string]string{"AWS_PROFILE": "readwrite", "AWS_DEFAULT_PROFILE": "readwrite"},
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

			log := logrus.New()
			log.SetOutput(io.Discard)

			clearStaleProfile(t.Context(), log)

			assert.Equal(t, os.Getenv("AWS_PROFILE"), tc.want)
			assert.Equal(t, os.Getenv("AWS_DEFAULT_PROFILE"), tc.wantDefault)
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
