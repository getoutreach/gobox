// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment

// Package cli contains various cli utilities that are useful for building
// cli applications with gobox based applications
package cli

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/logfile"
	"github.com/getoutreach/gobox/pkg/cli/updater"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Run runs a urface/cli application.
func Run(ctx context.Context, cancel context.CancelFunc, a *cli.App, conf *Config) {
	// If no logger is provided, use a discard logger.
	logger := conf.Logger
	if logger == nil {
		_logger := logrus.New()
		_logger.SetOutput(io.Discard)
		logger = _logger
	}

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

	if conf.Telemetry.UseDelibird {
		logger.Debug("Using delibird for telemetry")
		if err := logfile.Hook(); err != nil {
			logger.WithError(err).Warn("Failed to capture logs, continuing without logging to file")
		}
	}

	// Support loading compiled in keys from the binary through the
	// config framework
	overrideConfigLoaders(conf)

	// Cancel the context on ^C and other signals
	urfaveRegisterShutdownHandler(cancel)

	// Setup tracing, with a top-level span being the name of the application
	if err := trace.InitTracer(ctx, app.Info().Name); err != nil {
		logger.WithError(err).Warn("Failed to initialize tracer")
	}
	ctx = trace.StartSpan(ctx, app.Info().Name, trace.CommonProps())

	exitCode, exit := setupExitHandler(ctx)
	// Note: All defers before this point will not be ran because this will
	// call os.Exit()
	defer exit()
	defer trace.End(ctx)

	if _, err := updater.UseUpdater(ctx, updater.WithApp(a), updater.WithLogger(logger)); err != nil {
		logger.WithError(err).Warn("Failed to setup automatic updater")
	}

	cli.OsExiter = func(code int) { (*exitCode) = code }

	// Print a stack trace when a panic occurs and set the exit code
	defer setupPanicHandler(exitCode)
	ctx = trace.StartCall(ctx, "main")
	defer trace.EndCall(ctx)

	oldBefore := a.Before
	a.Before = func(c *cli.Context) error {
		if oldBefore != nil {
			if err := oldBefore(c); err != nil {
				return err
			}
		}
		return urfaveBefore(c)
	}

	if err := a.RunContext(ctx, os.Args); err != nil {
		logger.Errorf("failed to run: %v", err)
		//nolint:errcheck // Why: We're attaching the error to the trace.
		trace.SetCallStatus(ctx, err)
		(*exitCode) = 1

		return
	}
}

// HookInUrfaveCLI sets up an app.Before that automatically traces command runs
// and automatically updates itself.
//
// TODO(jaredallard): Deprecate this after templates have been updated and released
// for a few weeks.
func HookInUrfaveCLI(ctx context.Context, cancel context.CancelFunc, a *cli.App,
	logger logrus.FieldLogger, honeycombAPIKey, dataset, teleforkAPIKey string) {
	Run(ctx, cancel, a, &Config{
		Telemetry: TelemetryConfig{
			Otel: TelemetryOtelConfig{
				HoneycombAPIKey: cfg.SecretData(honeycombAPIKey),
				Dataset:         dataset,
				Debug:           false,
			},
		},
		Logger: logger,
	})
}

// urfaveBefore is a cli.BeforeFunc that implements tracing
func urfaveBefore(c *cli.Context) error {
	trace.AddInfo(c.Context, trace.CommonProps(), log.F{
		"cli.subcommand": c.Args().First(),
		"cli.args":       strings.Join(c.Args().Tail(), " "),
	})

	return nil
}
