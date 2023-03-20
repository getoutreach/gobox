// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains shell completion helpers.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/getoutreach/gobox/pkg/cli/updater"
	"github.com/urfave/cli/v2"
)

// Returns true if the last argument to the program is
// --generate-bash-completion, the special hidden flag urfave creates.
// See: https://cli.urfave.org/v2/#bash-completion
func isBashCompletion(lastArg string) bool {
	return lastArg == "--generate-bash-completion"
}

// Returns true if the last argument to the program is
// --generate-fish-completion, a special hidden flag gobox creates. This prints
// the results of ToFishCompletion to stdout.
// See: https://github.com/urfave/cli/blob/8b23e7b1e99f37934e029ef6c8d31efc7e744ff1/fish.go
func isFishCompletion(lastArg string) bool {
	return lastArg == "--generate-fish-completion"
}

// Attemps to generate shell completion.
// Returns an error if genration fails, or if the last of the given args doesn't
// match a completion flag.
func generateShellCompletion(ctx context.Context, a *cli.App, args []string) error {
	// First, inject the updater flags so that they show up in the help.
	a.Flags = append(a.Flags, updater.UpdaterFlags...)
	lastArg := args[len(args)-1]
	switch {
	case isBashCompletion(lastArg):
		// Shell completion is handled by urfave.
		return a.RunContext(ctx, args)
	case isFishCompletion(lastArg):
		// Print out fish completion.
		completion, err := a.ToFishCompletion()
		if err != nil {
			return err
		}
		writer := a.Writer
		if writer == nil {
			writer = os.Stdout
		}
		fmt.Fprintln(writer, completion)
		return nil
	default:
		return fmt.Errorf("generateShellCompletion called inappropriately")
	}
}
