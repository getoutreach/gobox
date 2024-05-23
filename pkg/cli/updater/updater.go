// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements an updater for CLIs that support multiple channels and version checks

// Package updater implements an updater for CLIs that support multiple channels and version checks
package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/briandowns/spinner"
	"github.com/fynelabs/selfupdate"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/github"
	"github.com/getoutreach/gobox/pkg/cli/updater/archive"
	"github.com/getoutreach/gobox/pkg/cli/updater/release"
	"github.com/getoutreach/gobox/pkg/cli/updater/resolver"
	"github.com/getoutreach/gobox/pkg/exec"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// Disabled globally disables the automatic updater.
// This is helpful for when using an external package manager, such as brew.
// This should usually be done with an ldflag:
//
//	go run -ldflags "-X github.com/getoutreach/gobox/pkg/cli/updater.Disabled=true" ...
//
// or you can do it in main() before UseUpdater is called:
//
//	updater.Disabled = "true"
//
// Any value other than "true" is considered false.
// The type is string to allow for invoking via `go run -ldflags "-X ..."`.
var Disabled = "false"

// UseUpdater creates an automatic updater.
func UseUpdater(ctx context.Context, opts ...Option) (*updater, error) { //nolint:revive // Why: Purposely not exported.
	u := &updater{}

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

	// disabledReason is the reason the updater is disabled
	disabledReason string

	// channel is the channel to use for checking for updates, this corresponds
	// to a git tag _or_ the pre-release field of versions, e.g. `0.1.0-alpha.1`
	// would be the channel `alpha`.
	channel string

	// channelReason is the reason why the channel is being used
	channelReason string

	// forceCheck forces the updater to check for updates regardless of the
	// last update check
	forceCheck bool

	// repo is the repository to check for updates, if this isn't
	// set then the version will be read from the embedded module information.
	// this requires go module to be used
	repoURL string

	// version is the current version of the application, if this isn't
	// set then the version will be read from the embedded module information.
	// this requires go module to be used.
	version string

	// executablePath is the path of the executable to update, defaults
	// to the current executable name. This is also used to determine the
	// binary to retrieve out of an archive.
	executablePath string

	// skipInstall skips the installation of the update if set
	skipInstall bool

	// checkInterval is the interval to check for updates
	checkInterval *time.Duration

	// skipMajorVersionPrompt auto-accepts the major version confirmation dialog
	// if set
	skipMajorVersionPrompt bool

	// noProgressBar disables the progress bar if set
	noProgressBar bool

	// app is a cli.App to setup commands on
	app *cli.App
}

// defaultOptions configures the default options for the updater if
// not already set
// nolint:funlen // Why: a lot of options to set
func (u *updater) defaultOptions() error {
	if Disabled == "true" {
		u.disabled = true
		u.disabledReason = "disabled via go linker"
		u.hookSkipUpdateIntoCLI()
		return nil
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

	if u.executablePath == "" {
		var err error
		u.executablePath, err = exec.ResolveExecuable(os.Args[0])
		if err != nil {
			u.hookSkipUpdateIntoCLI()
			return err
		}
	}

	// if repo isn't set, then we attempt to load it from the
	// go module debug information.
	if u.repoURL == "" {
		r, err := getRepoFromBuild()
		if err != nil {
			u.hookSkipUpdateIntoCLI()
			return fmt.Errorf("failed to determine which repository built this binary")
		}
		u.repoURL = "https://" + r
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
	if conf, err := readConfig(); err == nil {
		// If we don't have a channel, use the one from the config
		if u.channel == "" {
			if conf.GlobalConfig.Channel != "" {
				u.channel = conf.GlobalConfig.Channel
				u.channelReason = "configured in config (global)"
			}

			if repoConf, ok := conf.Get(u.repoURL); ok {
				u.channel = repoConf.Channel
				u.channelReason = "configured in config (repo level)"
			}
		} else {
			u.channelReason = "channel passed in to updater"
		}
	}

	curVersion, err := semver.NewVersion(u.version)
	if err != nil {
		u.disabled = true
		u.disabledReason = fmt.Sprintf("failed to parse current version as semver: %v", err)
	}

	if curVersion != nil {
		curChannel, curLocalBuild := u.getVersionInfo(curVersion)
		if curLocalBuild {
			u.disabled = true
			u.disabledReason = "using locally built version"
		}

		// determine channel from version string if not set
		if u.channel == "" {
			u.channel = curChannel
			u.channelReason = fmt.Sprintf("using %s version", curChannel)
		}
	}

	return nil
}

// getVersionInfo returns information about a version from
// a string. This is used to parse the version from the config file.
func (u *updater) getVersionInfo(v *semver.Version) (channel string, locallyBuilt bool) {
	splPre := strings.Split(v.Prerelease(), ".")

	// get the channel from the version string if set
	if len(splPre) > 0 && splPre[0] != "" {
		channel = splPre[0]

		// If the first char of the pre-release is a number then it's just
		// another locally built release, e.g. [2-gfe7ad99][0] -> 2
		if _, err := strconv.Atoi(channel[0:1]); err == nil {
			// we have no channel, so unset the previous value
			// (we had a number as the channel)
			channel = ""
			locallyBuilt = true
		}
	}

	// check if the build is a locally built version
	if len(splPre) >= 2 {
		// If the second part of the pre-release is _not_ a number then it's
		// a locally built release, e.g. [rc, 14-23-gfe7ad99][1]
		if _, err := strconv.Atoi(splPre[1]); err != nil {
			locallyBuilt = true
		}
	}

	// no channel defaults to stable
	if channel == "" {
		channel = resolver.StableChannel
	}

	return channel, locallyBuilt
}

// check checks for updates and applies them if necessary, returning true if
// an update was applied signifying the application should restart.
func (u *updater) check(ctx context.Context) (bool, error) {
	// Never update when device is not a terminal, or when in a CI environment. However,
	// we allow forceCheck to override this.
	if u.disabled || (!u.forceCheck && (!term.IsTerminal(int(os.Stdin.Fd())) || os.Getenv("CI") != "")) {
		return false, nil
	}

	conf, err := readConfig()
	if err != nil {
		u.log.WithError(err).Warn("failed to read config")
	}

	repoCache := conf.UpdaterCache[u.repoURL]

	// if we're not forcing an update, then check if we need to update
	// based on the check interval
	if !u.forceCheck {
		// We're ok with dereferencing the pointer here.
		checkAmount := *u.checkInterval
		if conf.GlobalConfig.CheckEvery != 0 {
			checkAmount = conf.GlobalConfig.CheckEvery
		}

		if repoConf, ok := conf.Get(u.repoURL); ok {
			if repoConf.CheckEvery != 0 {
				checkAmount = repoConf.CheckEvery
			}
		}

		// if we're not past the last update threshold
		// then we don't check for updates.
		//nolint:gocritic // Why: should not replace After with Before
		if !time.Now().After(repoCache.LastChecked.Add(checkAmount)) {
			return false, nil
		}
	}

	// Start the checking for updates spinner
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond,
		spinner.WithSuffix(" Checking for updates..."))
	spin.Start()

	v, err := resolver.Resolve(ctx, u.ghToken, &resolver.Criteria{
		URL:     u.repoURL,
		Channel: u.channel,
	})

	// We're done checking for updates at this point, stop!
	spin.Stop()

	if err != nil {
		return false, errors.Wrap(err, "failed to check for updates")
	}

	// update the cache with the latest check time
	repoCache.LastChecked = time.Now().UTC()
	conf.UpdaterCache[u.repoURL] = repoCache

	// write that we checked for updates
	if err := conf.Save(); err != nil {
		u.log.WithError(err).Warn("failed to save updater cache")
	}

	if should, reason := u.shouldUpdate(v); !should {
		u.log.WithField("reason", reason).Debug("no update available")
		return false, nil
	}

	relNotes, err := release.GetReleaseNotes(ctx, u.ghToken, &release.GetReleaseNoteOptions{
		RepoURL: u.repoURL,
		Tag:     v.Tag,
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to fetch release")
	}

	// handle major versions by prompting the user if this is one
	if !u.skipMajorVersionPrompt {
		if shouldContinue := handleMajorVersion(u.log, u.version, v.String(), relNotes); !shouldContinue {
			return false, nil
		}
	}

	if err := u.installVersion(ctx, v); err != nil {
		return false, errors.Wrap(err, "failed to install update")
	}

	// write our current version to the cache, as we just successfully updated
	repoCache.LastVersion = u.version
	conf.UpdaterCache[u.repoURL] = repoCache
	if err := conf.Save(); err != nil {
		u.log.WithError(err).Warn("failed to save updater cache")
		return true, nil
	}

	return true, nil
}

// shouldUpdate returns true if the updater should update and the reason
// why it should, or shouldn't as a string.
func (u *updater) shouldUpdate(v *resolver.Version) (bool, string) { //nolint:gocritic // Why: doc'd above
	curV, err := resolver.NewVersionFromVersionString(u.version)
	if err != nil {
		return false, fmt.Sprintf("failed to parse current version: %s", err)
	}

	if v.LessThan(curV) {
		return false, "newer version is less than current version"
	}

	if v.Equal(curV) {
		return false, "newer version is equal to current version"
	}

	return true, "version is newer than current version"
}

// installVersion installs a specific version of the application
func (u *updater) installVersion(ctx context.Context, v *resolver.Version) error {
	u.log.WithField("version", v.String()).Info("Installing update")
	a, aName, aSize, err := release.Fetch(ctx, u.ghToken, &release.FetchOptions{
		RepoURL: u.repoURL,
		Tag:     v.Tag,

		// Assets are uploaded starting with `repoName`.
		// Don't use filepath.Base(u.executableName),
		// because if the executable is not named the same as the repo
		// (and there is no restriction for it be so)
		// then the installer won't be able to find the asset.
		AssetName: filepath.Base(u.repoURL) + "_*_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.*",
	})
	if err != nil {
		return errors.Wrap(err, "failed to fetch release")
	}
	defer a.Close()

	// write to temp file for better user experience on download
	tmpF, err := os.CreateTemp("", aName)
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}
	defer tmpF.Close()
	defer os.Remove(tmpF.Name()) //nolint:errcheck // Why: Best effort

	var w io.Writer = tmpF
	if !u.noProgressBar {
		pb := progressbar.DefaultBytes(aSize, "Downloading Update")
		defer pb.Close()

		w = io.MultiWriter(tmpF, pb)
	}

	if _, err := io.Copy(w, a); err != nil {
		return errors.Wrap(err, "failed to download update")
	}

	// re-open the file to reset the position
	tmpF.Close() //no lint:errcheck // Why: Best effort
	tmpF, err = os.Open(tmpF.Name())
	if err != nil {
		return errors.Wrap(err, "failed to open temp file")
	}

	bin, header, err := archive.Extract(ctx, aName, tmpF,
		archive.WithFilePath(filepath.Base(u.executablePath)),
	)
	if err != nil {
		return errors.Wrap(err, "failed to extract release")
	}
	defer bin.Close()

	// skip install is mostly just for testing
	if u.skipInstall {
		return nil
	}

	var r io.Reader = bin
	if !u.noProgressBar {
		// There's an empty space here to make it align with the first progress bar.
		pb := progressbar.DefaultBytes(header.Size, "Extracting Update ")
		defer pb.Close()

		r = io.TeeReader(bin, pb)
	}

	return selfupdate.Apply(r, selfupdate.Options{})
}
