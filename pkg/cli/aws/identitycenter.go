// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: This file resolves AWS credentials through AWS IAM
// Identity Center, using `granted` for the login flow.

package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/getoutreach/gobox/pkg/box"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// RefreshMethodIdentityCenter is the box AWS refresh method that logs in
// through AWS IAM Identity Center.
const RefreshMethodIdentityCenter = "identity-center"

// Config returns an AWS config for the active profile, logging in to
// AWS Identity Center when there is no usable session.
func Config(ctx context.Context, log logrus.FieldLogger) (aws.Config, error) {
	b, err := box.LoadBox()
	if err != nil {
		return aws.Config{}, errors.Wrap(err, "could not load box config")
	}

	return configForBox(ctx, log, b, b.AWS.DefaultProfile)
}

// configForBox is Config for a box config that is already loaded. An
// empty profile leaves the choice to the SDK defaults.
func configForBox(ctx context.Context, log logrus.FieldLogger, b *box.Config, profile string) (aws.Config, error) {
	if staleProfileOverride(ctx, log) && profile == "" {
		profile = "default"
	}

	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	return ensureSession(ctx, log,
		func() (aws.Config, error) { return config.LoadDefaultConfig(ctx, opts...) },
		func() error { return grantedLogin(ctx, &b.AWS.IdentityCenter) },
	)
}

// refreshCredsViaIdentityCenter makes sure that an Identity Center
// session exists. Unlike okta-aws-cli, it writes nothing to the AWS
// credentials file: the profiles in ~/.aws/config read the SSO token
// cache through the SDK.
func refreshCredsViaIdentityCenter(
	ctx context.Context,
	copts *CredentialOptions,
	acopts *AuthorizeCredentialsOptions,
	b *box.Config,
) error {
	if copts == nil {
		copts = &CredentialOptions{
			Profile: b.AWS.DefaultProfile,
			Role:    b.AWS.DefaultRole,
		}
	}

	log := copts.Log
	if log == nil {
		log = logrus.New()
	}

	if acopts.Output != "" {
		return fmt.Errorf("refresh method %q does not support credential provider output", RefreshMethodIdentityCenter)
	}

	ic := &b.AWS.IdentityCenter
	if err := validateIdentityCenter(ic); err != nil {
		return err
	}

	if acopts.DryRun {
		log.Infof("Dry Run: granted %s", strings.Join(grantedLoginArgs(ic), " "))
		return nil
	}

	if acopts.Force {
		return grantedLogin(ctx, ic)
	}

	_, err := configForBox(ctx, log, b, copts.Profile)
	return err
}

// staleProfileOverride reports whether the AWS profile that the
// environment names is gone from the shared config. Tools that moved from
// okta-aws-cli remove the profiles it wrote, and the shell config that
// exports one is not ours to edit, so the leftover would otherwise fail
// every load. The caller then picks a profile itself, which takes
// priority over the environment. Anything other than a missing profile
// is not stale: a config that does not parse is not a profile that is
// gone.
func staleProfileOverride(ctx context.Context, log logrus.FieldLogger) bool {
	env, err := config.NewEnvConfig()
	if err != nil {
		log.WithError(err).Debug("Could not read the AWS environment, keeping the profile")
		return false
	}

	if env.SharedConfigProfile == "" {
		return false
	}

	// LoadSharedConfigProfile reads the default file paths rather than
	// the ones the environment overrides, so pass those in.
	_, err = config.LoadSharedConfigProfile(ctx, env.SharedConfigProfile,
		func(o *config.LoadSharedConfigOptions) {
			if env.SharedConfigFile != "" {
				o.ConfigFiles = []string{env.SharedConfigFile}
			}

			if env.SharedCredentialsFile != "" {
				o.CredentialsFiles = []string{env.SharedCredentialsFile}
			}
		})

	var notExist config.SharedConfigProfileNotExistError
	if !errors.As(err, &notExist) {
		return false
	}

	// The SDK reads AWS_PROFILE first, then AWS_DEFAULT_PROFILE.
	key := "AWS_PROFILE"
	if os.Getenv(key) == "" {
		key = "AWS_DEFAULT_PROFILE"
	}

	log.Warnf("%s names %q, which ~/.aws/config no longer holds. Ignoring it; "+
		"drop it from your shell config and use `assume <profile>` instead.", key, env.SharedConfigProfile)

	return true
}

// ensureSession loads the config and checks that its credentials
// resolve. A missing or expired token means the credential process
// behind it has nothing to read, so it logs in once and loads again.
func ensureSession(ctx context.Context, log logrus.FieldLogger,
	load func() (aws.Config, error), login func() error,
) (aws.Config, error) {
	cfg, err := load()
	if err != nil {
		return aws.Config{}, errors.Wrap(err, "could not load AWS config")
	}

	if _, err := cfg.Credentials.Retrieve(ctx); err == nil {
		return cfg, nil
	}

	log.Info("No usable AWS credentials, logging in to AWS Identity Center")

	if err := login(); err != nil {
		return aws.Config{}, errors.Wrap(err, "could not log in to AWS Identity Center")
	}

	// The config was loaded before the token existed, so load it again
	// rather than reusing the credential cache of the first attempt.
	cfg, err = load()
	if err != nil {
		return aws.Config{}, errors.Wrap(err, "could not load AWS config after logging in")
	}

	if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
		return aws.Config{}, errors.Wrap(err, "AWS credentials do not resolve after logging in")
	}

	return cfg, nil
}

// validateIdentityCenter checks that the box config holds what the login
// needs.
func validateIdentityCenter(ic *box.IdentityCenterConfig) error {
	if ic.StartURL == "" || ic.Region == "" {
		return errors.New("box config aws.identityCenter needs startURL and region")
	}

	return nil
}

// grantedLoginArgs is shared by the real login and the dry-run output.
func grantedLoginArgs(ic *box.IdentityCenterConfig) []string {
	return []string{"sso", "login", "--sso-region", ic.Region, "--sso-start-url", ic.StartURL}
}

// grantedLogin opens the Identity Center login flow. `granted sso login`
// only starts the browser round trip and writes no credentials of its
// own; a credential process reads the resulting token afterwards. Unlike
// `granted credential-process`, it prints no session credentials to
// stdout.
func grantedLogin(ctx context.Context, ic *box.IdentityCenterConfig) error {
	if err := validateIdentityCenter(ic); err != nil {
		return err
	}

	if _, err := exec.LookPath("granted"); err != nil {
		return errors.New("failed to find granted in PATH")
	}

	// nolint: gosec // Why: The arguments come from the box config.
	cmd := exec.CommandContext(ctx, "granted", grantedLoginArgs(ic)...)
	cmd.Stdin = os.Stdin
	// Keep stdout clean for callers that read credential provider JSON.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return errors.Wrap(cmd.Run(), "could not run `granted sso login`")
}
