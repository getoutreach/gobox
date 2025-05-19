// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the urfave/cli integration
// for the updater, agnostic of major version of the library.

package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/getoutreach/gobox/pkg/cli/updater/resolver"
	"github.com/pkg/errors"
)

// SkipFlag is a CLI flag used to skip the updater check.
var SkipFlag = BoolFlag{
	Name:  "skip-update",
	Usage: "Skips the updater check",
}

// ForceFlag is a CLI flag used to force the updater check.
var ForceFlag = BoolFlag{
	Name:  "force-update-check",
	Usage: "Force checking for an update",
}

func (u *updater) hookSkipUpdateIntoCLI() {
	// append the skip-update flag
	if u.app != nil {
		u.app.Flags = append(u.app.Flags, UpdaterFlags[0])
	}
}

// newUpdaterCommand creates a new Command that interacts with the updater
func newUpdaterCommand(u *updater) *Command {
	return &Command{
		Name:  "updater",
		Usage: "Commands for interacting with the built-in updater",
		Commands: []*Command{
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

// newRollbackCommand creates a new Command that rolls back to the previous version
// or to the specified version
func newRollbackCommand(u *updater) *Command {
	conf, _ := ReadConfig() //nolint:errcheck // Why: Handled below
	repoCache := conf.UpdaterCache[u.repoURL]

	return &Command{
		Name:  "rollback",
		Usage: "Rollback to the previous version",
		Flags: []Flag{
			&StringFlag{
				Name:  "version",
				Usage: "The version to rollback to",
				Value: repoCache.LastVersion,
			},
		},
		Action: func(ctx context.Context, c *CLICmd) error {
			ver := c.String("version")
			if ver == "" {
				return fmt.Errorf("no previous version to rollback to, must be set with --version")
			}

			return cliInstallVersion(ctx, u, ver, true)
		},
	}
}

// new creates a new Command that replaces the current binary with
// a specific version of the binary.
func newUseCommand(u *updater) *Command {
	return &Command{
		Name:  "use",
		Usage: "Use a specific version of the application",
		Flags: []Flag{
			&BoolFlag{
				Name:  "list",
				Usage: "List available versions",
			},
		},
		Action: func(ctx context.Context, c *CLICmd) error {
			if c.Bool("list") {
				if ghPath, err := exec.LookPath("gh"); ghPath == "" || err != nil {
					return errors.New("gh is not installed, please install it to use this command")
				}

				//nolint:gosec // Why: This is OK.
				cmd := exec.CommandContext(ctx, "gh", "-R", u.repoURL, "release",
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

			return cliInstallVersion(ctx, u, ver, false)
		},
	}
}

// newSetChannel creates a new Command that sets the channel
func newSetChannel(u *updater) *Command {
	return &Command{
		Name:  "set-channel",
		Usage: "Set the channel to check for updates",
		Flags: []Flag{
			&BoolFlag{
				Name:  "reset",
				Usage: "Reset the channel to the default",
			},
		},
		Action: func(ctx context.Context, c *CLICmd) error {
			conf, err := ReadConfig()
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

			versions, err := resolver.GetVersions(ctx, u.ghToken, u.repoURL, false)
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
			updated, err := u.check(ctx)
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

// newGetChannels creates a new Command that returns the channels for the
// current application
func newGetChannels(u *updater) *Command {
	return &Command{
		Name:  "get-channels",
		Usage: "Returns the valid channels",
		Action: func(ctx context.Context, c *CLICmd) error {
			versions, err := resolver.GetVersions(ctx, u.ghToken, u.repoURL, false)
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

// newStatusCommand creates a new Command that returns the current
// status of the updater
func newStatusCommand(u *updater) *Command {
	return &Command{
		Name:  "status",
		Usage: "Returns the current status of the updater",
		Flags: []Flag{
			&BoolFlag{
				Name:  "debug",
				Usage: "Show debug information",
			},
		},
		Action: func(ctx context.Context, c *CLICmd) error {
			conf, err := ReadConfig()
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
				fmt.Println()
				fmt.Println("Remote Information:")
				v, err := resolver.Resolve(ctx, u.ghToken, &resolver.Criteria{
					URL:     u.repoURL,
					Channel: u.channel,
				})
				if err != nil {
					return errors.Wrap(err, "failed to resolve latest version")
				}
				fmt.Println("  Latest Version:", v.String())
				shouldUpdate, reason := u.shouldUpdate(v)
				fmt.Printf("  Update Reason: %s (should: %s)\n", reason, strconv.FormatBool(shouldUpdate))

				fmt.Println()
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

	conf, err := ReadConfig()
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
