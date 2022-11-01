// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides miscellaneous helpers for the updater
package updater

import (
	"fmt"
	"path"
	"runtime/debug"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/charmbracelet/glamour"
	"github.com/manifoldco/promptui"
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

// handleMajorVersion prompts the user when a new major version is available, returns
// true if we should continue, or false if we shouldn't
func handleMajorVersion(log logrus.FieldLogger, curV, newV, relNotes string) bool {
	// we skip errors because the above logic already parsed these version strings
	cver, err := semver.NewVersion(curV)
	if err != nil {
		return true
	}

	nver, err := semver.NewVersion(newV)
	if err != nil {
		return true
	}

	// if the current major is less than the new release
	// major then just return
	if !(cver.Major() < nver.Major()) {
		return true
	}

	out := relNotes
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if err == nil {
		out, err = r.Render(out)
		if err != nil {
			log.WithError(err).Warn("Failed to render release notes, using raw release notes")
		}
	} else if err != nil {
		log.WithError(err).Warn("Failed to create markdown render, using raw release notes")
	}
	fmt.Println(out)

	log.Infof("Detected major version upgrade (%d -> %d). Would you like to upgrade?", cver.Major, nver.Major)
	prompt := promptui.Select{
		Label: "Select",
		Items: []string{"Yes", "No"},
	}
	_, resp, err := prompt.Run()
	if err != nil {
		return false
	}

	return strings.EqualFold(resp, "yes")
}
