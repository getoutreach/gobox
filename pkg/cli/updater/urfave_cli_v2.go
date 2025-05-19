// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description: urfave/cli/v3 updater command helpers.

package updater

import (
	"os"
	"runtime"
	"strings"
	"syscall"

	goboxexec "github.com/getoutreach/gobox/pkg/exec"
	cliV2 "github.com/urfave/cli/v2"
)

// The urfave (V2) flags the updater will inject.
var UpdaterFlags = []cliV2.Flag{
	SkipFlag.ToUrfaveV2(),
	ForceFlag.ToUrfaveV2(),
}

// hookIntoCLIV2 hooks into a urfave/cli/v2.App to add updater support
func (u *updater) hookIntoCLIV2() {
	oldBefore := u.app.Before

	// append the standard flags
	u.app.Flags = append(u.app.Flags, UpdaterFlags...)

	u.app.Before = func(c *cliV2.Context) error {
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
			return cliV2.Exit("", 0)
		}

		binPath, err := goboxexec.ResolveExecutable(os.Args[0])
		if err != nil {
			u.log.WithError(err).Warn("Failed to find binary location, please re-run your command manually")
			return cliV2.Exit("", 0)
		}

		u.log.Infof("%s has been updated, re-running automatically", u.app.Name)

		//nolint:gosec // Why: We're passing in os.Args
		if err := syscall.Exec(binPath, os.Args, os.Environ()); err != nil {
			return cliV2.Exit("failed to re-run binary, please re-run your command manually", 1)
		}

		return cliV2.Exit("", 0)
	}

	u.app.Commands = append(u.app.Commands, newUpdaterCommand(u).ToUrfaveV2())
}
