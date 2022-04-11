package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/getoutreach/gobox/pkg/cli/github"
	gogithub "github.com/google/go-github/v43/github"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

// lastUpdateCheck is information about the last time we checked for updates.
type lastUpdateCheck struct {
	Date       time.Time     `yaml:"date"`
	CheckEvery time.Duration `yaml:"checkEvery"`
}

// userConfig us user configuration for checking for updates
type userConfig struct {
	// AlwaysUsePrereleases instructs the updater to always consider prereleases
	AlwaysUsePrereleases bool `yaml:"alwaysUsePrereleases"`
}

// `NeedsUpdate` reads a GitHub token from `~/.outreach/github.token`. If it can't,
// it prompts the user and saves it to the path above. GitHub Releases is checked
// for a new release, and if it's found, updates the base binary. If an update is
// required, it returns `true`.
//
// If `repo` is not provided, it is determined via `debug.ReadBuildInfo` and
// the path the binary was built from. `repo` should be in the format of
// `getoutreach/$repoName`.
//
// `version` is usually the value of `gobox/pkg/app.Version`, which is set
// at build time.
//
// If `debugLog` is set to true, then the core updater's debug logging will be
// enabled. Deprecated: This should be set on the provided logger.
//
// Update checks can be disabled by setting `disabled` to true.
//nolint:funlen,gocyclo
func NeedsUpdate(ctx context.Context, log logrus.FieldLogger, repo, version string, disabled,
	debugLog, includePrereleases, forceCheck bool) bool {
	if disabled {
		return false
	}

	// Never update when device is not a terminal, or when in a CI environment. However,
	// we allow forceCheck to override this.
	if !forceCheck && (!term.IsTerminal(int(os.Stdin.Fd())) || os.Getenv("CI") != "") {
		return false
	}
	log = log.WithField("service", "updater")

	if repo == "" {
		r, err := getRepoFromBuild()
		if err != nil {
			log.WithError(err).Error("failed to determine which repository build this module")
			return false
		}
		repo = r
	}

	split := strings.Split(repo, "/")
	org := split[0]
	repoName := split[1]

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.WithError(err).Warn("failed to get user's home directory")
		return false
	}
	configDir := filepath.Join(homedir, ".outreach", ".updater")
	updateCheckPath := filepath.Join(configDir, org, repoName+".yaml")

	if userConf, err := readConfig(configDir); err == nil {
		if userConf.AlwaysUsePrereleases {
			includePrereleases = true
		}
	} else {
		log.WithError(err).Warn("failed to read user config")
	}

	if err := os.MkdirAll(filepath.Dir(updateCheckPath), 0o755); err != nil {
		log.WithError(err).Error("failed to create update metadata storage directory")
		return false
	}

	if !forceCheck {
		// check the last time we updated
		if b, err2 := os.ReadFile(updateCheckPath); err2 == nil {
			var last *lastUpdateCheck
			if err := yaml.Unmarshal(b, &last); err == nil {
				// if we're not past the last update thereshold
				// then we don't check for updates
				if !time.Now().After(last.Date.Add(last.CheckEvery)) {
					return false
				}
			} else {
				log.WithError(err).Warn("failed to parse last update information")
			}
		}
	}

	// Start the checking for updates spinner
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	spin.Suffix = " Checking for updates..."
	spin.Start()

	gh, err := github.NewClient()
	if err != nil {
		log.WithError(err).Warn("failed to create authenticated GitHub client")
		gh = gogithub.NewClient(nil)
	}

	g := NewGithubUpdaterWithClient(ctx, gh, org, repoName)
	r, err := g.GetLatestVersion(ctx, version, includePrereleases)
	if err != nil && !errors.Is(err, ErrNoNewRelease) {
		log.WithError(err).Warn("failed to check for updates")
		return false
	}

	// We're done checking for updates at this point, stop!
	spin.Stop()

	last := &lastUpdateCheck{
		Date:       time.Now(),
		CheckEvery: 30 * time.Minute,
	}

	// write that we checked for updates
	if b, err := yaml.Marshal(&last); err == nil {
		if err := os.WriteFile(updateCheckPath, b, 0o600); err != nil {
			log.WithError(err).Warn("failed to write update metadata")
		}
	} else if err != nil {
		log.WithError(err).Warn("failed to marshal update metadata")
	}

	// return here so that we were able to write that we found no new updates
	if errors.Is(err, ErrNoNewRelease) {
		return false
	}

	// handle major versions
	shouldContinue := handleMajorVersion(ctx, log, version, r)
	if !shouldContinue {
		return false
	}

	log.Infof("Downloading update %s", r.GetTagName())
	newBinary, cleanupFunc, err := g.DownloadRelease(ctx, r, repoName, "")
	defer cleanupFunc()
	if err != nil {
		log.WithError(err).Error("failed to download latest release")
		return false
	}

	log.Infof("Installing update (%s -> %s)", version, r.GetTagName())
	err = g.ReplaceRunning(ctx, newBinary)
	if err != nil && !errors.Is(err, &exec.ExitError{}) {
		log.WithError(err).Error("failed to install update")
	}

	return true
}

// readConfig reads the user's configuration from a well-known path
func readConfig(configDir string) (userConfig, error) {
	configPath := filepath.Join(configDir, "config.yaml")
	f, err := os.Open(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return userConfig{}, nil
	} else if err != nil {
		return userConfig{}, err
	}

	var config userConfig
	if err := yaml.NewDecoder(f).Decode(&config); err != nil {
		return userConfig{}, errors.Wrap(err, "failed to decode user config")
	}

	return config, nil
}

// getRepoFromBuild reads the repository from the embedded go module information
func getRepoFromBuild() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("failed to read build info, was this built with go module support")
	}

	repoName := strings.TrimPrefix(info.Main.Path, "github.com/")
	repoSplit := strings.Split(repoName, "/")

	if len(repoSplit) < 2 {
		return "", fmt.Errorf("failed to parse %v as a repository", repoName)
	}

	return path.Join(repoSplit[0], repoSplit[1]), nil
}

// handleMajorVersion prompts the user when a new major version is available
func handleMajorVersion(ctx context.Context, log logrus.FieldLogger, currentVersion string, rel *gogithub.RepositoryRelease) bool {
	// we skip errors because the above logic already parsed these version strings
	cver, err := semver.ParseTolerant(currentVersion)
	if err != nil {
		return true
	}

	nver, err := semver.ParseTolerant(rel.GetTagName())
	if err != nil {
		return true
	}

	// if the current major is less than the new release
	// major then just return
	if !(cver.Major < nver.Major) {
		return true
	}

	out := rel.GetBody()
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if err == nil {
		out, err = r.Render(rel.GetBody())
		if err != nil {
			log.WithError(err).Warn("Failed to render release notes, using raw release notes")
		}
	} else if err != nil {
		log.WithError(err).Warn("Failed to create markdown render, using raw release notes")
	}

	fmt.Println(out)

	log.Infof("Detected major version upgrade (%d -> %d). Would you like to upgrade?", cver.Major, nver.Major)
	shouldContinue, err := GetYesOrNoInput(ctx)
	if err != nil {
		return false
	}

	return shouldContinue
}
