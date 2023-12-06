// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file defines the functional arguments to the updater.

package updater

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Options configures an updater
type Option func(*updater)

// WithRepoURL sets the repository to use for checking for updates.
// The expected format is: https://<host>/<repo>
// Note: It should not contain .git at the end
func WithRepoURL(repo string) Option {
	return func(u *updater) {
		u.repoURL = repo
	}
}

// WithSkipMajorVersionPrompt sets whether or not to skip the prompt for
// major version upgrades
func WithSkipMajorVersionPrompt(skip bool) Option {
	return func(u *updater) {
		u.skipMajorVersionPrompt = skip
	}
}

// WithNoProgressBar sets whether or not to show the progress bar.
func WithNoProgressBar(noProgressBar bool) Option {
	return func(u *updater) {
		u.noProgressBar = noProgressBar
	}
}

// WithVersion sets the version to use as the current version when
// checking for updates. Defaults to app.Info().Version.
func WithVersion(version string) Option {
	return func(u *updater) {
		u.version = version
	}
}

// WithLogger sets the logger to use for logging. If not set
// a io.Discard logger is created.
func WithLogger(logger logrus.FieldLogger) Option {
	return func(u *updater) {
		u.log = logger
	}
}

// WithDisabled sets if we should disable the updater or not
func WithDisabled(disabled bool) Option {
	return func(u *updater) {
		u.disabled = disabled
	}
}

// Deprecated: Set the channel via the WithChannel option.
// WithPrereleases sets whether or not to include prereleases in the update check.
func WithPrereleases(_ bool) Option {
	return func(u *updater) {
		u.channel = "rc"
	}
}

// WithChannel sets the channel to use for checking for updates
func WithChannel(channel string) Option {
	return func(u *updater) {
		u.channel = channel
	}
}

// WithForceCheck sets whether or not to force the updater to check for updates
// otherwise updates are checked for only if the last check was more than
// the update check interval.
func WithForceCheck(forceCheck bool) Option {
	return func(u *updater) {
		u.forceCheck = forceCheck
	}
}

// WithApp sets the cli.App to setup commands on.
func WithApp(app *cli.App) Option {
	return func(u *updater) {
		u.app = app
	}
}

// WithCheckInterval sets the interval to check for updates.
// Defaults to 30 minutes.
func WithCheckInterval(interval time.Duration) Option {
	return func(u *updater) {
		u.checkInterval = &interval
	}
}

// WithSkipInstall sets whether or not to skip the installation of the update
func WithSkipInstall(skipInstall bool) Option {
	return func(u *updater) {
		u.skipInstall = skipInstall
	}
}

// WithExecutableName overrides the name of the executable. See u.executablePath.
// Deprecated: Use WithExecutablePath
func WithExecutableName(execName string) Option {
	return WithExecutablePath(execName)
}

// WithExecutablePath overrides the path of the executable. See u.executablePath.
func WithExecutablePath(execPath string) Option {
	return func(u *updater) {
		u.executablePath = execPath
	}
}
