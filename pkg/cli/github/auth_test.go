package github_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getoutreach/gobox/pkg/cli/github"
	"gotest.tools/v3/assert"
)

//nolint:gocritic // Why: It's obvious.
func setupEnv(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("OUTREACH_GITHUB_TOKEN", "")
	return tempDir
}

func Test_GetToken_outreachDirToken(t *testing.T) {
	home := setupEnv(t)

	dummyValue := "i wanna be the very best"

	oToken := filepath.Join(home, ".outreach", "github.token")
	assert.NilError(t, os.MkdirAll(filepath.Dir(oToken), 0o755),
		"expected mkdir setup to succeed")
	assert.NilError(t, os.WriteFile(oToken, []byte(dummyValue), 0o755),
		"expected writing token setup to succeed")

	token, err := github.GetToken()
	assert.NilError(t, err, "expected GetToken() to succeed")
	assert.Equal(t, string(token), dummyValue, "expected set token to be returned by GetToken()")
}

func Test_GetToken_ghCLIToken(t *testing.T) {
	home := setupEnv(t)

	t.Setenv("GOBOX_SKIP_VALIDATE_AUTH", "true")

	dummyValue := "like no one ever was"
	fakeYAML := "github.com:\n  user: jaredallard\n  oauth_token: " + dummyValue

	oToken := filepath.Join(home, ".config", "gh", "hosts.yml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(oToken), 0o755),
		"expected mkdir setup to succeed")
	assert.NilError(t, os.WriteFile(oToken, []byte(fakeYAML), 0o755),
		"expected writing token setup to succeed")

	token, err := github.GetToken()
	assert.NilError(t, err, "expected GetToken() to succeed")
	assert.Equal(t, string(token), dummyValue, "expected set token to be returned by GetToken()")
}

func Test_GetToken_envToken(t *testing.T) {
	setupEnv(t)

	dummyValue := "to catch them is my real test"

	t.Setenv("GITHUB_TOKEN", dummyValue)

	token, err := github.GetToken()

	assert.NilError(t, err, "expected GetToken() to succeed")
	assert.Equal(t, string(token), dummyValue, "expected set token to be returned by GetToken()")
}
