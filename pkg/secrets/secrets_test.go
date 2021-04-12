package secrets_test

import (
	"context"
	"strings"
	"testing"

	_ "github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/secrets"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
	"github.com/getoutreach/gobox/pkg/shuffler"
)

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestSecretsDevEnvRedirect(t *testing.T) {
	defer secretstest.Fake("/etc/mysecret", "fake value")()

	ctx := context.Background()
	if val, err := secrets.Config(ctx, "/etc/mysecret"); err != nil || val != "fake value" {
		t.Fatal("unexpected config failure", err, val)
	}
}

func (suite) TestSecretsFetchFile(t *testing.T) {
	ctx := context.Background()

	if x := secrets.MustConfig(ctx, "testdata/sample.txt"); strings.TrimSpace(x) != "sample" {
		t.Error("Secrets fetch failed", x)
	}
}
