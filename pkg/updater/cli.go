// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the urfave/cli.App integration
// for the updater.

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	goboxexec "github.com/getoutreach/gobox/pkg/exec"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// hookIntoCLI hooks into a urfave/cli.App to add updater support
func (u *updater) hookIntoCLI() {
	oldBefore := u.app.Before

	// append the standard flags
	u.app.Flags = append(u.app.Flags, []cli.Flag{
		&cli.BoolFlag{
			Name:  "skip-update",
			Usage: "skips the updater check",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "enables debug logging for all components that use logrus",
		},
		&cli.BoolFlag{
			Name:  "enable-prereleases",
			Usage: "Enable considering pre-releases when checking for updates",
		},
		&cli.BoolFlag{
			Name:  "force-update-check",
			Usage: "Force checking for an update",
		},
	}...)

	u.app.Before = func(c *cli.Context) error {
		// Handle deprecations and parse the flags onto our updater struct
		for _, f := range c.FlagNames() {
			if strings.EqualFold(f, "enable-prereleases") {
				u.prereleases = true
				u.log.Warn("--enable-prereleases is deprecated, use the new 'updater set-channel prereleases' command instead")
			}

			if strings.EqualFold(f, "force-update-check") {
				u.forceCheck = true
			}

			if strings.EqualFold(f, "skip-updater") {
				u.disabled = true
			}
		}

		// handle an older before if it was set
		if oldBefore != nil {
			if err := oldBefore(c); err != nil {
				return err
			}
		}

		// restart when updated
		needsUpdate, err := u.check(c.Context)
		if err != nil {
			u.log.WithError(err).Warn("Failed to handle updates")
			return nil
		}
		if !needsUpdate {
			return nil
		}

		switch runtime.GOOS {
		case "linux", "darwin":
			// handle later
		default:
			u.log.Infof("%s has been updated, please re-run your command", u.app.Name)
			return cli.Exit("", 0)
		}

		binPath, err := goboxexec.ResolveExecuable(os.Args[0])
		if err != nil {
			u.log.WithError(err).Warn("Failed to find binary location, please re-run your command manually")
			return cli.Exit("", 0)
		}

		u.log.Infof("%s has been updated, re-running automatically", u.app.Name)

		//nolint:gosec // Why: We're passing in os.Args
		if err := syscall.Exec(binPath, os.Args, os.Environ()); err != nil {
			return cli.Exit("failed to re-run binary, please re-run your command manually", 1)
		}

		return cli.Exit("", 0)
	}

	u.app.Commands = append(u.app.Commands, newUpdaterCommand(u))
}

// newUpdaterCommand creates a new cli.Command that interacts with the updater
func newUpdaterCommand(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "updater",
		Usage: "Commands for interacting with the built-in updater",
		Subcommands: []*cli.Command{
			newSetChannel(u),
			newRollbackCommand(u),
			newListReleases(u),
		},
	}
}

// newRollbackCommand creates a new cli.Command that rolls back to the previous version
// or to the specified version
func newRollbackCommand(u *updater) *cli.Command {
	last, _ := loadLastUpdateCheck(u.repo) //nolint:errcheck // Why: Handled below
	if last == nil {
		// we're going to require user input via --version
		last = &lastUpdateCheck{}
	}

	return &cli.Command{
		Name:  "rollback",
		Usage: "Rollback to the previous version",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "version",
				Usage: "The version to rollback to",
				Value: last.PreviousVersion,
			},
		},
		Action: func(c *cli.Context) error {
			version := c.String("version")
			if version == "" {
				return fmt.Errorf("no previous version to rollback to, must be set with --version")
			}

			u.log.Infof("Rolling back to %s", version)

			org, repoName, err := getOrgRepoFromString(u.repo)
			if err != nil {
				return errors.Wrap(err, "failed to get org and repo name")
			}

			g := NewGithubUpdaterWithClient(c.Context, u.gh, org, repoName)
			r, err := g.GetRelease(c.Context, version)
			if err != nil {
				return errors.Wrapf(err, "failed to find version %q", version)
			}

			newBinary, cleanupFunc, err := g.DownloadRelease(c.Context, r, repoName, "")
			defer cleanupFunc()
			if err != nil {
				return errors.Wrap(err, "failed to download release")
			}

			if err := g.ReplaceRunning(c.Context, newBinary); err != nil && !errors.Is(err, &exec.ExitError{}) {
				return errors.Wrap(err, "failed to install update")
			}

			u.log.Info("Rollback complete")
			return nil
		},
	}
}

// newListReleases creates a new cli.Command that lists the releases for the
// current application
func newListReleases(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "list-releases",
		Usage: "List all releases",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"L"},
				Usage:   "The number of releases to list",
				Value:   50,
			},
		},
		Action: func(c *cli.Context) error {
			if ghPath, err := exec.LookPath("gh"); ghPath == "" || err != nil {
				return errors.New("gh is not installed, please install it to use this command")
			}

			//nolint:gosec // Why: This is OK.
			cmd := exec.CommandContext(c.Context, "gh", "-R", u.repo, "release",
				"list", "-L", strconv.Itoa(c.Int("limit")))
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			cmd.Stdin = os.Stdin
			return cmd.Run()
		},
	}
}

// newSetChannel creates a new cli.Command that sets the channel
func newSetChannel(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "set-channel",
		Usage: "Set the channel to check for updates: release or rc",
		Action: func(c *cli.Context) error {
			channel := strings.ToLower(c.Args().Get(0))
			if channel == "" {
				return fmt.Errorf("channel must be provided")
			}

			if channel != "release" && channel != "rc" {
				return fmt.Errorf("channel must be either 'release' or 'rc'")
			}

			conf, err := readConfig(u.repo)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return errors.Wrap(err, "failed to read the config")
				}
				conf = userConfig{}
			}

			conf.AlwaysUsePrereleases = channel == "rc"
			if err := conf.Save(); err != nil {
				return errors.Wrap(err, "failed to save the config")
			}

			u.forceCheck = true
			updated, err := u.check(c.Context)
			if err != nil {
				return errors.Wrap(err, "failed to check for updates")
			}

			if !updated {
				return nil
			}

			u.log.Infof("Updated to latest %q version", channel)
			return nil
		},
	}
}
