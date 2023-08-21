// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file is the bulk of the package

// Package aws contains helpers for working with AWS in CLIs
package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/getoutreach/gobox/pkg/box"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/versent/saml2aws/v2/pkg/awsconfig"
)

// CredentialOptions configures what credentials are provided
type CredentialOptions struct {
	// Log is a logger to be used for informational logging.
	// if not supplied no output aside from prompting will be displayed
	Log logrus.FieldLogger

	// Role to assume for the user
	Role string

	// Profile to use
	Profile string

	// FileName is the name of the file to use for storing
	// AWS credentials. Defaults to `~/.aws/credentials`.
	FileName string
}

// DefaultCredentialOptions uses the default role and profile
// for accessing AWS.
func DefaultCredentialOptions() *CredentialOptions {
	b, err := box.LoadBox()
	if err != nil {
		return nil
	}

	return &CredentialOptions{
		Role:    b.AWS.DefaultRole,
		Profile: b.AWS.DefaultProfile,
	}
}

// assumedToRole takes an assumed-role arn and converts it to the
// arn of the role that was assumed
func assumedToRole(assumedRole string) string {
	spl := strings.Split(assumedRole, "/")
	if len(spl) != 3 {
		return assumedRole
	}

	spl[0] = strings.Replace(strings.Replace(spl[0], "assumed-role", "role", 1), "sts", "iam", 1)
	return strings.Join(spl[:2], "/")
}

// needsRefresh determines if AWS authentication needs to be refreshed
// or setup.
func needsRefresh(copts *CredentialOptions) (needsNewCreds bool, reason string) {
	if creds, err := awsconfig.NewSharedCredentials(copts.Profile, copts.FileName).Load(); err == nil {
		// Check, via the principal_arn, if the creds match the role we want
		if creds.PrincipalARN != "" && assumedToRole(creds.PrincipalARN) != copts.Role {
			return true, "Refreshing AWS credentials due to existing credentials using a different role"
		}

		// If our have no expiration date, it's probably not set. So, attempt to refresh.
		if creds.Expires.IsZero() {
			return true, "No existing credentials"
		}

		// Attempt to refresh the aws credentials via saml2aws if
		// they can expire. If they can refresh within 10 minutes of
		// the expiration period or if they are expired.
		if time.Now().Add(10 * time.Minute).After(creds.Expires) {
			return true, "Credentials are expired"
		}
	} else {
		// Failed to load the config, so attempt to refresh
		return true, fmt.Sprintf("Credential file failed to load: %v", err)
	}

	// Default to creds being valid
	return false, ""
}

// EnsureValidCredentials ensures that the current AWS credentials are valid
// and if they can expire it is attempted to rotate them when they are expired
// via saml2aws
func EnsureValidCredentials(ctx context.Context, copts *CredentialOptions) error { //nolint:funlen,lll // Why: cleaner to keep everything together
	if _, ok := os.LookupEnv("CI"); ok {
		return nil
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		copts.Log.Debug("Skipping AWS credentials refresh check, AWS_ACCESS_KEY_ID is set")
		return nil
	}

	if copts == nil {
		copts = DefaultCredentialOptions()
	}

	needsNewCreds, reason := needsRefresh(copts)
	if needsNewCreds { // Refresh the credentials
		b, err := box.LoadBox()
		if err != nil {
			return errors.Wrap(err, "could not load refresh credential config")
		}
		switch b.AWS.RefreshMethod {
		case "okta-aws-cli":
			if err := refreshCredsViaOktaAWSCLI(ctx, copts, reason); err != nil {
				return err
			}
		case "saml2aws":
		case "":
			if err := refreshCredsViaSaml2aws(ctx, copts, reason); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown AWS refresh method '%s'", b.AWS.RefreshMethod)
		}
	}

	return nil
}

func refreshCredsViaOktaAWSCLI(ctx context.Context, copts *CredentialOptions, reason string) error {
	if _, err := exec.LookPath("okta-aws-cli"); err != nil {
		return fmt.Errorf("failed to find okta-aws-cli in PATH")
	}

	if copts.Log != nil {
		copts.Log.WithField("reason", reason).Info("Obtaining AWS credentials via Okta")
	}

	//nolint:gosec // Why: What other option do I have
	cmd := exec.CommandContext(ctx,
		"okta-aws-cli",
		"--open-browser",
		"--write-aws-credentials",
		"--profile",
		copts.Profile,
		"--aws-iam-role",
		copts.Role,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to refresh AWS credentials via okta-aws-cli")
	}

	return nil
}

func refreshCredsViaSaml2aws(ctx context.Context, copts *CredentialOptions, reason string) error {
	if _, err := exec.LookPath("saml2aws"); err != nil {
		return fmt.Errorf("failed to find saml2aws, please run orc setup")
	}

	if copts.Log != nil {
		copts.Log.WithField("reason", reason).Info("Obtaining AWS credentials via Okta")
	}

	//nolint:gosec // Why: What other option do I have
	cmd := exec.CommandContext(ctx, "saml2aws", "login", "--profile", copts.Profile, "--role", copts.Role, "--force")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to refresh AWS credentials via saml2aws")
	}

	return nil
}
