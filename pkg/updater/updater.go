package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/blang/semver/v4"
	"github.com/briandowns/spinner"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/github"
	"github.com/getoutreach/gobox/pkg/updater/resolver"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// Disabled globally disables the automatic updater. This is helpful
// for when using an external package manager, such as brew.
// This should usually be done with an ldflag.
var Disabled bool

// This block contains directories the updater uses
var (
	// cacheDir is the directory where we store the cache of releases
	cacheDir = filepath.Join(".outreach", ".cache", "updater")

	// configDir is the directory where we store the user's configuration for
	// the updater per repository.
	configDir = filepath.Join(".outreach", ".config")
)

// UseUpdater creates an automatic updater.
func UseUpdater(ctx context.Context, opts ...Option) (*updater, error) { //nolint:revive // Why: Purposely not exported.
	u := &updater{
		disabled: Disabled,
	}

	// parse the provided options
	for _, opt := range opts {
		opt(u)
	}

	// set the default options as needed
	if err := u.defaultOptions(); err != nil {
		return nil, err
	}

	return u, nil
}

// updater is an updater that updates the current running binary to the latest
type updater struct {
	// ghToken is the GitHub token to use for the updater.
	ghToken cfg.SecretData

	// log is the logger to use for logging
	log logrus.FieldLogger

	// disabled disables the updater if set
	disabled bool

	// channel is the channel to use for checking for updates, this corresponds
	// to a git tag _or_ the pre-release field of versions, e.g. `0.1.0-alpha.1`
	// would be the channel `alpha`.
	channel string

	// forceCheck forces the updater to check for updates regardless of the
	// last update check
	forceCheck bool

	// repo is the repository to check for updates, if this isn't
	// set then the version will be read from the embedded module information.
	// this requires go module to be used.
	//
	// The format is expected to be the go import path of a repository, e.g.
	// https://github.com/getoutreach/devenv -> github.com/getoutreach/devenv
	repo string

	// version is the current version of the application, if this isn't
	// set then the version will be read from the embedded module information.
	// this requires go module to be used.
	version string

	// checkInterval is the interval to check for updates
	checkInterval *time.Duration

	// app is a cli.App to setup commands on
	app *cli.App
}

// defaultOptions configures the default options for the updater if
// not already set
func (u *updater) defaultOptions() error {
	if u.log == nil {
		// create a null output logger, we're not going to use it
		// if a logger wasn't passed in. This is to prevent panics.
		log := logrus.New()
		log.Out = io.Discard
		u.log = log
	}

	if u.checkInterval == nil {
		defaultDur := 30 * time.Minute
		u.checkInterval = &defaultDur
	}

	if u.version == "" {
		u.version = app.Info().Version
	}

	// if repo isn't set, then we attempt to load it from the
	// go module debug information.
	if u.repo == "" {
		r, err := getRepoFromBuild()
		if err != nil {
			return fmt.Errorf("failed to determine which repository built this binary")
		}
		u.repo = r
	}

	if string(u.ghToken) == "" {
		token, err := github.GetToken()
		if err != nil {
			token = ""
		}
		u.ghToken = token
	}

	// setup the updater
	if u.app != nil {
		u.hookIntoCLI()
	}

	// read the user's config and mutate the options based on that
	// if certain values are present
	if userConf, err := readConfig(u.repo); err == nil {
		// always use the user's channel, if they have one
		if userConf.Channel != "" {
			u.channel = userConf.Channel
		}
	}

	// determine channel from version string if not set
	if u.channel == "" {
		curVersion, err := semver.ParseTolerant(u.version)
		if err != nil {
			return errors.Wrapf(err, "failed to parse current version %q as semver", u.version)
		}

		// If we don't have a channel, but we have a PreRelease field, use that.
		// e.g. v0.1.0-alpha.1 -> alpha
		if len(curVersion.Pre) > 0 {
			u.channel = curVersion.Pre[0].String()
		}
	}

	return nil
}

// check checks for updates and applies them if necessary, returning true if
// an update was applied signifying the application should restart.
func (u *updater) check(ctx context.Context) (bool, error) {
	// Never update when device is not a terminal, or when in a CI environment. However,
	// we allow forceCheck to override this.
	if u.disabled || (!u.forceCheck && (!term.IsTerminal(int(os.Stdin.Fd())) || os.Getenv("CI") != "")) {
		return false, nil
	}

	last, err := loadLastUpdateCheck(u.repo)
	if !u.forceCheck {
		if err == nil {
			// if we're not past the last update threshold
			// then we don't check for updates
			if !time.Now().After(last.Date.Add(last.CheckEvery)) {
				return false, nil
			}
		} else {
			u.log.WithError(err).Warn("failed to read last update check")
		}
	}

	// Start the checking for updates spinner
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond,
		spinner.WithSuffix(" Checking for updates..."))
	spin.Start()

	v, err := resolver.Resolve(ctx, u.ghToken, &resolver.Criteria{
		URL:     "https://" + u.repo,
		Channel: u.channel,
	})

	// We're done checking for updates at this point, stop!
	spin.Stop()

	if err != nil {
		return false, errors.Wrap(err, "failed to check for updates")
	}

	last.Date = time.Now().UTC()
	last.CheckEvery = *u.checkInterval

	// write that we checked for updates
	if err := last.Save(); err != nil {
		u.log.WithError(err).Warn("failed to save updater cache")
	}

	// If the latest version is equal to what we have right now, then we don't need to update.
	if v.String() == u.version {
		return false, nil
	}

	// handle major versions by prompting the user if this is one
	// if shouldContinue := handleMajorVersion(ctx, u.log, u.version, r); !shouldContinue {
	// 	return false, nil
	// }

	// if err := u.installVersion(ctx, r); err != nil {
	// 	return false, errors.Wrap(err, "failed to install update")
	// }

	return true, nil
}
