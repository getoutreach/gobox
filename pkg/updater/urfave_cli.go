// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the urfave/cli.App integration
// for the updater.

package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	goboxexec "github.com/getoutreach/gobox/pkg/exec"
	"github.com/getoutreach/gobox/pkg/updater/resolver"
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
			Name:  "force-update-check",
			Usage: "Force checking for an update",
		},
	}...)

	u.app.Before = func(c *cli.Context) error {
		// Handle deprecations and parse the flags onto our updater struct
		for _, f := range c.FlagNames() {
			if strings.EqualFold(f, "force-update-check") {
				u.forceCheck = true
			}

			if strings.EqualFold(f, "skip-update") {
				u.disabled = true
			}
		}

		// Skip the updater if we're running an updater command, that we provide.
		if c.Args().First() == "updater" {
			u.disabled = true
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
			// We handle these after the switch.
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
			newGetChannel(u),
			newGetChannels(u),
			newRollbackCommand(u),
			newUseCommand(u),
		},
	}
}

// cliInstallVersion is shared code to install/rollback and application version
func cliInstallVersion(ctx context.Context, u *updater, version string, rollback bool) error {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	str := "Rolling back to"
	if !rollback {
		str = "Installing"
	}
	u.log.Infof("%s %s", str, version)
	if err := u.installVersion(ctx, version); err != nil {
		return err
	}
	str = "Rollback complete"
	if !rollback {
		str = "Installation complete"
	}
	u.log.Info(str)

	return nil
}

// newRollbackCommand creates a new cli.Command that rolls back to the previous version
// or to the specified version
func newRollbackCommand(u *updater) *cli.Command {
	conf, _ := readConfig() //nolint:errcheck // Why: Handled below
	repoCache := conf.UpdaterCache[u.repoURL]

	return &cli.Command{
		Name:  "rollback",
		Usage: "Rollback to the previous version",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "version",
				Usage: "The version to rollback to",
				Value: repoCache.LastVersion,
			},
		},
		Action: func(c *cli.Context) error {
			ver := c.String("version")
			if ver == "" {
				return fmt.Errorf("no previous version to rollback to, must be set with --version")
			}

			return cliInstallVersion(c.Context, u, ver, true)
		},
	}
}

// new creates a new cli.Command that replaces the current binary with
// a specific version of the binary.
func newUseCommand(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "use",
		Usage: "Use a specific version of the application",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "list",
				Usage: "List available versions",
			},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("list") {
				if ghPath, err := exec.LookPath("gh"); ghPath == "" || err != nil {
					return errors.New("gh is not installed, please install it to use this command")
				}

				//nolint:gosec // Why: This is OK.
				cmd := exec.CommandContext(c.Context, "gh", "-R", u.repoURL, "release",
					"list", "-L", "20")
				cmd.Stderr = os.Stderr
				cmd.Stdout = os.Stdout
				cmd.Stdin = os.Stdin
				return cmd.Run()
			}

			ver := c.Args().First()
			if ver == "" {
				return fmt.Errorf("no version specified")
			}

			return cliInstallVersion(c.Context, u, ver, false)
		},
	}
}

// newSetChannel creates a new cli.Command that sets the channel
func newSetChannel(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "set-channel",
		Usage: "Set the channel to check for updates",
		Action: func(c *cli.Context) error {
			channel := strings.ToLower(c.Args().Get(0))
			if channel == "" {
				return fmt.Errorf("channel must be provided")
			}

			versions, err := resolver.GetVersions(c.Context, u.ghToken, u.repoURL)
			if err != nil {
				return errors.Wrap(err, "failed to determine channels from remote")
			}

			if _, ok := versions[channel]; !ok {
				return fmt.Errorf("channel %q is not valid, run 'get-channels' to return a list of valid channels", channel)
			}

			conf, err := readConfig()
			if err != nil {
				return errors.Wrap(err, "failed to read config")
			}

			repoConf, _ := conf.Get(u.repoURL)
			repoConf.Channel = channel
			conf.Set(u.repoURL, repoConf)

			if err := conf.Save(); err != nil {
				return errors.Wrap(err, "failed to save the config")
			}

			u.forceCheck = true
			u.channel = channel
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

// newGetChannel creates a new cli.Command that sets the channel
func newGetChannel(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "get-channel",
		Usage: "Returns the current channel",
		Action: func(c *cli.Context) error {
			fmt.Println(u.channel)
			return nil
		},
	}
}

// newGetChannels creates a new cli.Command that returns the channels for the
// current application
func newGetChannels(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "get-channels",
		Usage: "Returns the valid channels",
		Action: func(c *cli.Context) error {
			versions, err := resolver.GetVersions(c.Context, u.ghToken, u.repoURL)
			if err != nil {
				return errors.Wrap(err, "failed to determine channels from remote")
			}

			for channel := range versions {
				fmt.Println(channel)
			}
			return nil
		},
	}
}
