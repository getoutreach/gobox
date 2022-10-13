// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment

// Package cli contains various cli utilities that are useful for building
// cli applications with gobox based applications
package cli

import (
	"context"
	"io"
	"os"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cli/updater"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// intPtr turns an int into a *int
func intPtr(i int) *int {
	return &i
}

// HookInUrfaveCLI sets up an app.Before that automatically traces command runs
// and automatically updates itself.
//
//nolint:funlen // Why: Also not worth doing at the moment, we split a lot of this out already.
func HookInUrfaveCLI(ctx context.Context, cancel context.CancelFunc, a *cli.App,
	logger logrus.FieldLogger, honeycombAPIKey, dataset, teleforkAPIKey string) {
	// Quick exit if this is asking for a shell completion. We do this before
	// setting up any hooks or checking for updates to keep things speedy.
	lastArg := os.Args[len(os.Args)-1]
	if a.EnableBashCompletion && (isBashCompletion(lastArg) || isFishCompletion(lastArg)) {
		if err := generateShellCompletion(ctx, a, os.Args); err != nil {
			// This will be invisible to the user, most likely - but log for
			// debugging.
			logger.Errorf("failed to generate completion: %v", err)
		}

		return
	}

	env.ApplyOverrides()
	app.SetName(a.Name)

	// Ensure that we don't use the standard outreach logger
	log.SetOutput(io.Discard)

	// Support loading compiled in keys from the binary through the
	// config framework
	overrideConfigLoaders(honeycombAPIKey, dataset, false)

	// Cancel the context on ^C and other signals
	urfaveRegisterShutdownHandler(cancel)

	exitCode, exit := setupExitHandler(ctx)
	defer exit()

	if _, err := updater.UseUpdater(ctx, updater.WithApp(a), updater.WithLogger(logger)); err != nil {
		logger.WithError(err).Warn("Failed to setup automatic updater")
	}

	cli.OsExiter = func(code int) { (*exitCode) = code }

	// Print a stack trace when a panic occurs and set the exit code
	defer setupPanicHandler(exitCode)

	if err := a.RunContext(ctx, os.Args); err != nil {
		logger.Errorf("failed to run: %v", err)
		//nolint:errcheck // Why: We're attaching the error to the trace.
		trace.SetCallStatus(ctx, err)
		(*exitCode) = 1

		return
	}
}
