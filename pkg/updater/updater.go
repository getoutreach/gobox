package updater

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

type LastUpdateCheck struct {
	Date       time.Time     `yaml:"date"`
	CheckEvery time.Duration `yaml:"checkEvery"`
}

func GetUserInput() (string, error) {
	reader := bufio.NewReader(os.Stdin) //create new reader, assuming bufio imported
	return reader.ReadString('\n')
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
// enabled.
//
// Update checks can be disabled by setting `disabled` to true.
//nolint:funlen,gocyclo
func NeedsUpdate(ctx context.Context, log logrus.FieldLogger, repo, version string, disabled,
	debugLog, includePrereleases, forceCheck bool) bool {
	if disabled {
		return false
	}

	// Never update when device is not a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	log = log.WithField("service", "updater")

	if repo == "" {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			log.Warn("failed to read build info, was this built with go modules?")
			return false
		}

		repoName := strings.TrimPrefix(info.Main.Path, "github.com/")
		repoSplit := strings.Split(repoName, "/")

		if len(repoSplit) < 2 {
			log.Warn("failed to parse repository name ", repoName)
			return false
		}

		repo = fmt.Sprintf("%s/%s", repoSplit[0], repoSplit[1])
	}

	split := strings.Split(repo, "/")
	org := split[0]
	repoName := split[1]

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.WithError(err).Warn("failed to get user's home directory")
		return false
	}

	updateCheckPath := filepath.Join(homedir, ".outreach", ".updater", org, repoName+".yaml")
	tokenPath := filepath.Join(homedir, ".outreach", "github.token")

	err = os.MkdirAll(filepath.Dir(updateCheckPath), 0755)
	if err != nil {
		log.WithError(err).Error("failed to create update metadata storage directory")
		return false
	}

	if !forceCheck {
		// check the last time we updated
		if b, err2 := ioutil.ReadFile(updateCheckPath); err2 == nil {
			var last *LastUpdateCheck
			err2 = yaml.Unmarshal(b, &last)
			if err2 == nil {
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

	// This token is not safe for usage, it will be logged if accidentally
	// set to be logged. DO NOT USE. Use `token` instead.
	var unsafeToken string
	if b, err2 := ioutil.ReadFile(tokenPath); err2 != nil {
		unsafeToken, err2 = saveNewToken(log, tokenPath)
		if err2 != nil {
			log.WithError(err2).Warn("failed to persist token to disk, we wil ask for it again")
		}
	} else {
		// we had no error, so process the token
		unsafeToken = string(b)
	}
	token := cfg.SecretData(strings.TrimSpace(unsafeToken))

	g := NewGithubUpdater(ctx, log, token, org, repoName)
	r, err := g.GetLatestVersion(ctx, version, includePrereleases)
	if err != nil && err != ErrNoNewRelease {
		log.WithError(err).Warn("failed to check for updates")
		return false
	}

	last := &LastUpdateCheck{
		Date:       time.Now(),
		CheckEvery: 30 * time.Minute,
	}

	// write that we checked for updates
	if b, err2 := yaml.Marshal(&last); err2 == nil {
		err2 = ioutil.WriteFile(updateCheckPath, b, 0600)
		if err2 != nil {
			log.WithError(err2).Warn("failed to write update metadata")
		}
	} else if err2 != nil {
		log.WithError(err2).Warn("failed to marshal update metadata")
	}

	// return here so that we were able to write that we found no new updates
	if err == ErrNoNewRelease {
		return false
	}

	// handle major versions
	shouldContinue := handleMajorVersion(ctx, log, version, r.GetTagName(), repo)
	if !shouldContinue {
		return false
	}

	log.Infof("Downloading update %v", r.GetTagName())
	newBinary, cleanupFunc, err := g.DownloadRelease(ctx, r, repoName, "")
	defer cleanupFunc()
	if err != nil {
		log.WithError(err).Error("failed to download latest release")
		return false
	}

	log.Infof("Installing update (%v -> %v)", version, r.GetTagName())
	err = g.ReplaceRunning(ctx, newBinary)
	if err != nil && !errors.Is(err, &exec.ExitError{}) {
		log.WithError(err).Error("failed to install update")
	}

	return true
}

func handleMajorVersion(ctx context.Context, log logrus.FieldLogger, currentVersion, newVersion, repo string) bool {
	// we skip errors because the above logic already parsed these version strings
	cver, err := semver.ParseTolerant(currentVersion)
	if err != nil {
		return true
	}

	nver, err := semver.ParseTolerant(newVersion)
	if err != nil {
		return true
	}

	// if the current major is less than the new release
	// major then just return
	if !(cver.Major < nver.Major) {
		return true
	}

	log.Infof("Detected major version upgrade (%d -> %d). Would you like to upgrade?", cver.Major, nver.Major)
	log.Infof("Release notes are available here: https://github.com/%s/releases/%s", repo, newVersion)
	shouldContinue, err := GetYesOrNoInput(ctx)
	if err != nil {
		return false
	}

	return shouldContinue
}

func saveNewToken(log logrus.FieldLogger, tokenPath string) (string, error) {
	log.Infoln("We need a GitHub Personal Access Token in order to automatically update this tool.")
	log.Infoln("Instructions: https://outreach-io.atlassian.net/wiki/spaces/EN/pages/784041501")
	log.Infoln("Please enter your personal Github Access Token: ")

	token, err := GetUserInput()
	if err != nil {
		return "", errors.Wrap(err, "failed to get user input")
	}

	err = os.MkdirAll(filepath.Dir(tokenPath), 0755)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create token dir '%v'", tokenPath)
	}

	err = ioutil.WriteFile(tokenPath, []byte(token), 0600)
	if err != nil {
		log.WithError(err).Warn("failed to save github access token into keyring")
	}

	return token, nil
}
