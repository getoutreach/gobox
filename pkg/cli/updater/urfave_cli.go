// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the urfave/cli.App integration
// for the updater.

package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/getoutreach/gobox/pkg/cli/updater/resolver"
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
				u.disabledReason = "skip-update flag set"
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
			newGetChannels(u),
			newRollbackCommand(u),
			newUseCommand(u),
			newStatusCommand(u),
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
	if err := u.installVersion(ctx, &resolver.Version{
		Tag: version,
	}); err != nil {
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
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "reset",
				Usage: "Reset the channel to the default",
			},
		},
		Action: func(c *cli.Context) error {
			conf, err := readConfig()
			if err != nil {
				return errors.Wrap(err, "failed to read config")
			}

			if c.Bool("reset") {
				delete(conf.PerRepositoryConfiguration, u.repoURL)
				if err := conf.Save(); err != nil {
					return errors.Wrap(err, "failed to save config")
				}

				fmt.Println("Reset channel to default (or global config) value")
				return nil
			}

			channel := strings.ToLower(c.Args().First())
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

			repoConf, _ := conf.Get(u.repoURL)
			repoConf.Channel = channel
			conf.Set(u.repoURL, &repoConf)
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

// newStatusCommand creates a new cli.Command that returns the current
// status of the updater
func newStatusCommand(u *updater) *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Returns the current status of the updater",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Show debug information",
			},
		},
		Action: func(c *cli.Context) error {
			conf, err := readConfig()
			if err != nil {
				return errors.Wrap(err, "failed to read config")
			}

			disabled := (u.disabled && !u.forceCheck)
			lastCheck := conf.UpdaterCache[u.repoURL].LastChecked
			nextCheck := lastCheck.Add(*u.checkInterval)
			status := "enabled"
			if disabled {
				status = fmt.Sprintf("disabled (%s)", u.disabledReason)
			} else if u.forceCheck {
				status = "enabled (forced)"
			}

			fmt.Println("Version:", u.version)
			fmt.Printf("Channel: %s (%s)\n", u.channel, u.channelReason)
			fmt.Println("Updater Status:", status)

			lastCheckStr := lastCheck.Format(time.RFC1123)
			if lastCheck.IsZero() {
				lastCheckStr = "never"
			}

			fmt.Println("Last Update Check:", lastCheckStr)
			if !disabled {
				fmt.Println("")
				fmt.Printf("Checking for updates again at %s\n", color.New(color.Bold).Sprint(nextCheck.Format(time.RFC1123)))
			}

			if c.Bool("debug") {
				if err := printDebug(u); err != nil {
					return errors.Wrap(err, "failed to print debug information")
				}
			}

			return nil
		},
	}
}

// printDebug prints debug information about the updater
func printDebug(u *updater) error {
	fmt.Println("")
	fmt.Println("Updater Struct")
	fmt.Println("=================")

	// Safety first Cooper
	u.ghToken = "***"
	sConf := spew.NewDefaultConfig()
	sConf.MaxDepth = 1
	sConf.Dump(u)

	fmt.Println("")
	fmt.Println("Config")
	fmt.Println("=================")

	conf, err := readConfig()
	if err != nil {
		return errors.Wrap(err, "failed to read config")
	}
	spew.Dump(conf)

	fmt.Println("")
	fmt.Println("-- Raw:")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to determine home directory")
	}

	b, err := os.ReadFile(filepath.Join(homeDir, ConfigFile))
	if err != nil {
		return errors.Wrap(err, "failed to read config file")
	}
	fmt.Println(string(b))

	return nil
}
