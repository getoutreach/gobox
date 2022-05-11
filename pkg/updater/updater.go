package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cli/github"
	gogithub "github.com/google/go-github/v43/github"
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
//nolint:revive // Why: Purposely not exported.
func UseUpdater(ctx context.Context, opts ...Option) (*updater, error) {
	u := &updater{
		disabled: Disabled,
	}

	// parse the provided options
	for _, opt := range opts {
		opt(u)
	}

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
			return nil, fmt.Errorf("failed to determine which repository built this binary")
		}
		u.repo = r
	}

	gh, err := github.NewClient()
	if err != nil {
		gh = gogithub.NewClient(nil)
	}
	u.gh = gh

	// setup the updater
	if u.app != nil {
		u.hookIntoCLI()
	}

	return u, nil
}

// Deprecated: Use UseUpdater instead.
// NeedsUpdate is a deprecated method of UseUpdater.
func NeedsUpdate(ctx context.Context, log logrus.FieldLogger, repo, version string, disabled,
	debugLog, includePrereleases, forceCheck bool) bool {
	u, err := UseUpdater(ctx, WithLogger(log), WithRepo(repo), WithVersion(version),
		WithDisabled(disabled), WithPrereleases(includePrereleases), WithForceCheck(forceCheck))
	if err != nil {
		return false
	}

	needsUpdate, err := u.check(ctx)
	if err != nil {
		return false
	}
	return needsUpdate
}

// Options configures an updater
type Option func(*updater)

// updater is an updater that updates the current running binary to the latest
type updater struct {
	// gh is the github client
	gh *gogithub.Client

	// log is the logger to use for logging
	log logrus.FieldLogger

	// disabled disables the updater if set
	disabled bool

	// prereleases is whether or not to include prereleases in the update check
	prereleases bool

	// forceCheck forces the updater to check for updates regardless of the
	// last update check
	forceCheck bool

	// repo is the repository to check for updates, if this isn't
	// set then the version will be read from the embedded module information.
	// this requires go module to be used.
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

// installVersion installs a specific version of the application.
func (u *updater) installVersion(ctx context.Context, r *gogithub.RepositoryRelease) error {
	org, repoName, err := getOrgRepoFromString(u.repo)
	if err != nil {
		return errors.Wrap(err, "failed to get org and repo name")
	}

	g := NewGithubUpdaterWithClient(ctx, u.gh, org, repoName)

	u.log.Infof("Downloading update %s", r.GetTagName())
	newBinary, cleanupFunc, err := g.DownloadRelease(ctx, r, repoName, "")
	defer cleanupFunc()
	if err != nil {
		return errors.Wrap(err, "failed to download latest release")
	}

	u.log.Infof("Installing update (%s -> %s)", u.version, r.GetTagName())
	err = g.ReplaceRunning(ctx, newBinary)
	if err != nil && !errors.Is(err, &exec.ExitError{}) {
		u.log.WithError(err).Error("failed to install update")
	}

	last, err := loadLastUpdateCheck(u.repo)
	if err != nil {
		return errors.Wrap(err, "failed to load last update check")
	}

	// save the version we're on as being the last version used
	last.PreviousVersion = u.version

	if err := last.Save(); err != nil {
		u.log.WithError(err).Warn("failed to persist last version to updater cache")
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

	org, repoName, err := getOrgRepoFromString(u.repo)
	if err != nil {
		return false, errors.Wrap(err, "failed to get org and repo name")
	}

	if userConf, err := readConfig(u.repo); err == nil {
		if userConf.AlwaysUsePrereleases {
			u.prereleases = true
		}
	} else {
		u.log.WithError(err).Warn("failed to read user config")
	}

	last, err := loadLastUpdateCheck(u.repo)
	if !u.forceCheck {
		if err == nil {
			// if we're not past the last update thereshold
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

	g := NewGithubUpdaterWithClient(ctx, u.gh, org, repoName)
	r, latestVersionError := g.GetLatestVersion(ctx, u.version, u.prereleases)
	if latestVersionError != nil && !errors.Is(latestVersionError, ErrNoNewRelease) {
		return false, errors.Wrap(latestVersionError, "failed to check for updates")
	}

	// We're done checking for updates at this point, stop!
	spin.Stop()

	last.Date = time.Now().UTC()
	last.CheckEvery = *u.checkInterval

	// write that we checked for updates
	if err := last.Save(); err != nil {
		u.log.WithError(err).Warn("failed to save updater cache")
	}

	// return here so that we were able to write that we found no new updates
	if errors.Is(latestVersionError, ErrNoNewRelease) {
		return false, nil
	}

	// handle major versions by prompting the user if this is one
	if shouldContinue := handleMajorVersion(ctx, u.log, u.version, r); !shouldContinue {
		return false, nil
	}

	if err := u.installVersion(ctx, r); err != nil {
		return false, errors.Wrap(err, "failed to install update")
	}

	return true, nil
}
