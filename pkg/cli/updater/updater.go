package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/briandowns/spinner"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/github"
	"github.com/getoutreach/gobox/pkg/cli/updater/archive"
	"github.com/getoutreach/gobox/pkg/cli/updater/release"
	"github.com/getoutreach/gobox/pkg/cli/updater/resolver"
	"github.com/getoutreach/gobox/pkg/exec"
	"github.com/inconshreveable/go-update"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// Disabled globally disables the automatic updater. This is helpful
// for when using an external package manager, such as brew.
// This should usually be done with an ldflag.
var Disabled bool

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

	// executableName is the name of the executable to update, defaults
	// to the current executable name. This is also used to determine the
	// binary to retrieve out of an archive.
	executableName string

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
func (u *updater) defaultOptions() error {
	if Disabled {
		u.disabled = true
		u.disabledReason = "disabled via go linker"
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

	curVersion, err := semver.ParseTolerant(u.version)
	if err != nil {
		return errors.Wrapf(err, "failed to parse current version %q as semver", u.version)
	}

	if u.executableName == "" {
		var err error
		u.executableName, err = exec.ResolveExecuable(os.Args[0])
		if err != nil {
			return err
		}
	}

	// if repo isn't set, then we attempt to load it from the
	// go module debug information.
	if u.repoURL == "" {
		r, err := getRepoFromBuild()
		if err != nil {
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
		if conf.GlobalConfig.Channel != "" {
			u.channel = conf.GlobalConfig.Channel
		}

		if repoConf, ok := conf.Get(u.repoURL); ok {
			u.channel = repoConf.Channel
		}
	}

	// determine channel from version string if not set
	if u.channel == "" {
		// If we don't have a channel, but we have a PreRelease field, use that.
		// e.g. v0.1.0-alpha.1 -> alpha
		if len(curVersion.Pre) > 0 {
			u.channel = curVersion.Pre[0].String()
			u.channelReason = fmt.Sprintf("using %s release", u.channel)
		} else {
			// default to the stable channel if we don't have a channel
			u.channel = "stable"
			u.channelReason = "using stable version"
		}
	} else {
		u.channelReason = "using channel from config"
	}

	// Disable the updater if we have >= 2 pre-releases in
	// our version string, for example:
	//  Skips: v10.3.0-rc.14-23-gfe7ad99 -> [rc, 14-23-gfe7ad99]
	//  But not: v10.3.0-rc.14
	//  But not: v0.0.0-unstable+fe7ad99f422422abb97d9104aac54259d3a1c9b4 (+ is build metadata)
	if len(curVersion.Pre) >= 2 {
		u.disabled = true
		u.disabledReason = "using locally built version"
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

	// If the latest version is equal to what we have right now, then we don't need to update.
	if v.String() == u.version {
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

	if err := u.installVersion(ctx, v.Tag); err != nil {
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

// installVersion installs a specific version of the application
func (u *updater) installVersion(ctx context.Context, tag string) error {
	a, aName, aSize, err := release.Fetch(ctx, u.ghToken, &release.FetchOptions{
		RepoURL: u.repoURL,
		Tag:     tag,
		// Note: If we're ever supporting azure devops or some other setup we might
		// need to change this logic and pull it into release?
		AssetNames: generatePossibleAssetNames(filepath.Base(u.repoURL), tag),
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
		archive.WithFilePath(filepath.Base(u.executableName)),
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

	return update.Apply(r, update.Options{})
}

// generatePossibleAssetNames generates a list of possible asset names for the
// given version. This is used to find the right asset to download.
func generatePossibleAssetNames(name, version string) []string {
	seperators := []string{"_", "-"}
	extensions := []string{".tar.xz", ".tar.gz", ".tar.bz2", ".zip"}
	versions := []string{version}
	if strings.HasPrefix(version, "v") {
		versions = append(versions, strings.TrimPrefix(version, "v"))
	}

	names := []string{}
	for _, v := range versions {
		for _, sep := range seperators {
			for _, ext := range extensions {
				// name[_-]version[_-]linux[_-]arm64[ext]
				names = append(names, name+sep+v+sep+runtime.GOOS+sep+runtime.GOARCH+ext)
			}
		}
	}

	return names
}
