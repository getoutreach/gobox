package github_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getoutreach/gobox/pkg/cli/github"
	"gotest.tools/v3/assert"
)

func setupEnv(t *testing.T) (tempDir string, cleanup func()) {
	envVars := []string{"HOME", "GITHUB_TOKEN", "OUTREACH_GITHUB_TOKEN"}
	origValues := make(map[string]string)
	for _, k := range envVars {
		v := os.Getenv(k)
		t.Logf("Cleared env var %s", k)
		os.Setenv(k, "") // set to empty
		origValues[k] = v
	}

	var err error
	tempDir, err = os.MkdirTemp("", "gobox-github-auth-*")
	assert.NilError(t, err, "expected test setup mkdir to succeed")

	cleanup = func() {
		assert.NilError(t, os.RemoveAll(tempDir), "expected test cleanup to succeed")
		for k, v := range origValues {
			t.Logf("Resetting env var %s=%v", k, v)
			os.Setenv(k, v)
		}
	}

	// set HOME to the temp dir, undone by cleanup()
	os.Setenv("HOME", tempDir)
	return
}

func Test_GetToken_outreachDirToken(t *testing.T) {
	home, cleanup := setupEnv(t)
	defer cleanup()

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
	home, cleanup := setupEnv(t)
	defer cleanup()

	os.Setenv("GOBOX_SKIP_VALIDATE_AUTH", "true")

	dummyValue := "like no one ever was"
	fakeYAML := "github.com:\n  user: jaredallard\n  oauth_token: " + dummyValue

	oToken := filepath.Join(home, ".config", "gh", "hosts.yml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(oToken), 0o755),
		"expected mkdir setup to succeed")
	assert.NilError(t, os.WriteFile(oToken, []byte(fakeYAML), 0o755),
		"expected writing token setup to succeed")

	token, err := github.GetToken()
	os.Setenv("GOBOX_SKIP_VALIDATE_AUTH", "")
	assert.NilError(t, err, "expected GetToken() to succeed")
	assert.Equal(t, string(token), dummyValue, "expected set token to be returned by GetToken()")
}

func Test_GetToken_envToken(t *testing.T) {
	_, cleanup := setupEnv(t)
	defer cleanup()

	dummyValue := "to catch them is my real test"

	os.Setenv("GITHUB_TOKEN", dummyValue)

	token, err := github.GetToken()
	os.Setenv("GITHUB_TOKEN", "")

	assert.NilError(t, err, "expected GetToken() to succeed")
	assert.Equal(t, string(token), dummyValue, "expected set token to be returned by GetToken()")
}
