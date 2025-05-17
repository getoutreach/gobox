// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description: urfave/cli/v3 updater command helpers.

package updater

import (
	"context"
	"os"
	"runtime"
	"strings"
	"syscall"

	goboxexec "github.com/getoutreach/gobox/pkg/exec"
	cliV2 "github.com/urfave/cli/v2"
	cliV3 "github.com/urfave/cli/v3"
)

// The urfave (V3) flags the updater will inject.
var UpdaterFlagsV3 = []cliV3.Flag{
	SkipFlag.ToUrfaveV3(),
	ForceFlag.ToUrfaveV3(),
}

// hookIntoCLIV3 hooks into a urfave/cli/v3.App to add updater support
func (u *updater) hookIntoCLIV3() {
	oldBefore := u.appV3.Before

	// append the standard flags
	u.appV3.Flags = append(u.appV3.Flags, UpdaterFlagsV3...)

	u.appV3.Before = func(ctx context.Context, c *cliV3.Command) (context.Context, error) {
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
			// nolint:govet // Why: the shadowing is intentional.
			if ctx, err := oldBefore(ctx, c); err != nil {
				return ctx, err
			}
		}

		// restart when updated
		needsUpdate, err := u.check(ctx)
		if err != nil {
			u.log.WithError(err).Warn("Failed to handle updates")
			return ctx, nil
		}
		if !needsUpdate {
			return ctx, nil
		}

		switch runtime.GOOS {
		case "linux", "darwin":
			// We handle these after the switch.
		default:
			u.log.Infof("%s has been updated, please re-run your command", u.appV3.Name)
			return ctx, cliV2.Exit("", 0)
		}

		binPath, err := goboxexec.ResolveExecutable(os.Args[0])
		if err != nil {
			u.log.WithError(err).Warn("Failed to find binary location, please re-run your command manually")
			return ctx, cliV2.Exit("", 0)
		}

		u.log.Infof("%s has been updated, re-running automatically", u.appV3.Name)

		//nolint:gosec // Why: We're passing in os.Args
		if err := syscall.Exec(binPath, os.Args, os.Environ()); err != nil {
			return ctx, cliV3.Exit("failed to re-run binary, please re-run your command manually", 1)
		}

		return ctx, cliV3.Exit("", 0)
	}

	u.appV3.Commands = append(u.appV3.Commands, newUpdaterCommand(u).ToUrfaveV3())
}
