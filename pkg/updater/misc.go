package updater

import (
	"context"
	"fmt"
	"path"
	"runtime/debug"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/charmbracelet/glamour"
	gogithub "github.com/google/go-github/v47/github"
	"github.com/sirupsen/logrus"
)

// getRepoFromBuild reads the repository from the embedded go module information
func getRepoFromBuild() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("failed to read build info, was this built with go module support")
	}

	// split on / so we can try to ignore a major version at the end
	spl := strings.Split(info.Main.Path, "/")
	if len(spl) < 3 {
		return "", fmt.Errorf("failed to parse repository from build info (len less than 3)")
	}

	// github.com getoutreach devenv -> github.com/getoutreach/devenv
	return path.Join(spl[0], spl[1], spl[2]), nil
}

// getOrgRepoFromString returns the org and repo from a string
// expected format: org/repo
func getOrgRepoFromString(s string) (string, string, error) { //nolint:gocritic // Why: This is in the function signature
	split := strings.Split(s, "/")
	if len(split) < 2 {
		return "", "", fmt.Errorf("failed to parse %v as a repository", s)
	}
	return split[0], split[1], nil
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
