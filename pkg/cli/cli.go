// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment

// Package cli contains various utilities that are useful for building
// CLI applications with gobox based applications. Support is available
// for both urfave/cli/v2 and urfave/cli/v3.
package cli

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cleanup"
	"github.com/getoutreach/gobox/pkg/cli/logfile"
	"github.com/getoutreach/gobox/pkg/cli/updater"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/sirupsen/logrus"
	cliV2 "github.com/urfave/cli/v2"
	cliV3 "github.com/urfave/cli/v3"
)

// ensureLogger ensures that the logger is set up correctly.
// If no logger is provided from the config object, it will create a
// discard logger.
func ensureLogger(conf *Config) logrus.FieldLogger {
	logger := conf.Logger
	if logger == nil {
		_logger := logrus.New()
		_logger.SetOutput(io.Discard)
		logger = _logger
	}

	return logger
}

// setupRun is a function that sets up the run context for the
// application, regardless of CLI library.
func setupRun(
	ctx context.Context,
	cancel context.CancelFunc,
	conf *Config,
	logger logrus.FieldLogger,
	name string,
) (cFuncs *cleanup.Funcs, exitCode *int, osExiter func(int)) {
	cFuncs = &cleanup.Funcs{}

	env.ApplyOverrides()
	app.SetName(name)

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
	// Note: All defers before this point will not be run because this will
	// call os.Exit()
	endTrace := func() {
		trace.End(ctx)
	}
	panicHandler := func() {
		// Print a stack trace when a panic occurs and set the exit code
		setupPanicHandler(exitCode)
	}
	*cFuncs = append(*cFuncs, &exit, &endTrace, &panicHandler)

	osExiter = func(code int) { (*exitCode) = code }

	return cFuncs, exitCode, osExiter
}

// runFailure is called when the CLI library returns an error.
func runFailure(ctx context.Context, logger logrus.FieldLogger, exitCode *int, err error) {
	logger.Errorf("failed to run: %v", err)
	//nolint:errcheck // Why: We're attaching the error to the trace.
	trace.SetCallStatus(ctx, err)
	(*exitCode) = 1
}

// Run runs a urfave/cli/v2 application.
func Run(ctx context.Context, cancel context.CancelFunc, a *cliV2.App, conf *Config) {
	logger := ensureLogger(conf)

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

	cFuncs, exitCode, osExiter := setupRun(ctx, cancel, conf, logger, a.Name)
	defer cFuncs.All()()

	if _, err := updater.UseUpdater(ctx, updater.WithApp(a), updater.WithLogger(logger)); err != nil {
		logger.WithError(err).Warn("Failed to setup automatic updater")
	}

	cliV2.OsExiter = osExiter

	ctx = trace.StartCall(ctx, "main")
	defer trace.EndCall(ctx)

	oldBefore := a.Before
	a.Before = func(c *cliV2.Context) error {
		if oldBefore != nil {
			if err := oldBefore(c); err != nil {
				return err
			}
		}
		return urfaveBefore(c)
	}

	if err := a.RunContext(ctx, os.Args); err != nil {
		runFailure(ctx, logger, exitCode, err)
	}
}

// RunV3 runs a urfave/cli/v3 application.
func RunV3(ctx context.Context, cancel context.CancelFunc, c *cliV3.Command, conf *Config) {
	logger := ensureLogger(conf)

	// Quick exit if this is asking for a shell completion. We do this before
	// setting up any hooks or checking for updates to keep things speedy.
	lastArg := os.Args[len(os.Args)-1]
	if c.EnableShellCompletion && lastArg == "--generate-shell-completion" {
		// Inject the updater flags so that they show up in the help.
		c.Flags = append(c.Flags, updater.UpdaterFlagsV3...)
		if err := c.Run(ctx, os.Args); err != nil {
			// This will be invisible to the user, most likely - but log for
			// debugging.
			logger.Errorf("failed to generate completion: %v", err)
		}

		return
	}

	cFuncs, exitCode, osExiter := setupRun(ctx, cancel, conf, logger, c.Name)
	defer cFuncs.All()()

	if _, err := updater.UseUpdater(ctx, updater.WithAppV3(c), updater.WithLogger(logger)); err != nil {
		logger.WithError(err).Warn("Failed to setup automatic updater")
	}

	cliV3.OsExiter = osExiter

	ctx = trace.StartCall(ctx, "main")
	defer trace.EndCall(ctx)

	oldBefore := c.Before
	c.Before = func(ctx context.Context, c *cliV3.Command) (context.Context, error) {
		if oldBefore != nil {
			// nolint:govet // Why: the shadowing is intentional.
			if ctx, err := oldBefore(ctx, c); err != nil {
				return ctx, err
			}
		}
		return urfaveV3Before(ctx, c)
	}

	if err := c.Run(ctx, os.Args); err != nil {
		runFailure(ctx, logger, exitCode, err)
	}
}

// HookInUrfaveCLI sets up a V2 app.Before func that automatically
// traces command runs & automatically updates itself.
//
// TODO(jaredallard): Deprecate this after templates have been updated and released
// for a few weeks.
func HookInUrfaveCLI(ctx context.Context, cancel context.CancelFunc, a *cliV2.App,
	logger logrus.FieldLogger, honeycombAPIKey, dataset, _ string) {
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

// urfaveBefore is a cli.BeforeFunc (V2) that implements tracing.
func urfaveBefore(c *cliV2.Context) error {
	trace.AddInfo(c.Context, trace.CommonProps(), log.F{
		"cli.subcommand": c.Args().First(),
		"cli.args":       strings.Join(c.Args().Tail(), " "),
	})

	return nil
}

// urfaveBefore is a cli.BeforeFunc (V3) that implements tracing.
func urfaveV3Before(ctx context.Context, c *cliV3.Command) (context.Context, error) {
	trace.AddInfo(ctx, trace.CommonProps(), log.F{
		"cli.subcommand": c.Args().First(),
		"cli.args":       strings.Join(c.Args().Tail(), " "),
	})

	return ctx, nil
}
