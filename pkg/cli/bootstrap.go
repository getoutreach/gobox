// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains cli functions used in bootstrap
// and eventually in stencil.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/exec"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/secrets"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/updater"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

// UpdateExitCode is the exit code returned when an update ocurred
const UpdateExitCode = 5

// overrideConfigLoaders fakes certain parts of the config that usually get pulled
// in via mechanisms that don't make sense to use in CLIs.
func overrideConfigLoaders(honeycombAPIKey, dataset string, tracingDebug bool) {
	// override the secret loader so that we can read specific keys from variables
	// otherwise fallback to the original secret loader, if it was set.
	var fallbackSecretLookup func(context.Context, string) ([]byte, error)
	fallbackSecretLookup = secrets.SetDevLookup(func(ctx context.Context, path string) ([]byte, error) {
		// use the embedded in value
		if path == "APIKey" {
			return []byte(honeycombAPIKey), nil
		}

		// if no fallback, return an error, failed to find :(
		// note: as of this time the secrets logic looks for
		// the path before falling back to the devlookup so this
		// is safe to assume all attempts have failed
		if fallbackSecretLookup == nil {
			return nil, fmt.Errorf("failed to find secret at path '%s', or compiled into binary", path)
		}

		return fallbackSecretLookup(ctx, path)
	})

	fallbackConfigReader := cfg.DefaultReader()
	cfg.SetDefaultReader(func(fileName string) ([]byte, error) {
		if fileName == "trace.yaml" {
			traceConfig := &trace.Config{
				Honeycomb: trace.Honeycomb{
					Enabled: true,
					APIHost: "https://api.honeycomb.io",
					APIKey: cfg.Secret{
						Path: "APIKey",
					},
					Debug:         tracingDebug,
					Dataset:       dataset,
					SamplePercent: 100,
				},
			}
			b, err := yaml.Marshal(&traceConfig)
			if err != nil {
				panic(err)
			}
			return b, nil
		}

		return fallbackConfigReader(fileName)
	})
}

// intPtr turns an int into a *int
func intPtr(i int) *int {
	return &i
}

// funcPtr turns a func into a *func
func funcPtr(fn func()) *func() {
	return &fn
}

// urfaveRegisterShutdownHandler registers a signal notifier that translates various term
// signals into context cancel
func urfaveRegisterShutdownHandler(cancel context.CancelFunc) {
	// handle ^C gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-c
		signal.Reset()
		cancel()
	}()
}

// setupTracer sets up a root trace for the CLI and initializes the tracer
func setupTracer(ctx context.Context, name string) context.Context {
	if err := trace.InitTracer(ctx, name); err != nil {
		fmt.Println(err)
		return ctx
	}
	return trace.StartTrace(ctx, name)
}

// setupPanicHandler sets up a panic handler for CLIs
func setupPanicHandler(exitCode *int) {
	if r := recover(); r != nil {
		fmt.Printf("stacktrace from panic: %s\n%s\n", r, string(debug.Stack()))

		// Go sets panic exit codes to 2
		(*exitCode) = 2
	}
}

// setupExitHandler sets up an exit handler
func setupExitHandler(ctx context.Context) (exitCode *int, exit func(), cleanup *func()) {
	exitCode = intPtr(0)
	cleanup = funcPtr(func() {})
	exit = func() {
		trace.End(ctx)
		trace.CloseTracer(ctx)
		if cleanup != nil {
			(*cleanup)()
		}
		os.Exit(*exitCode)
	}

	return
}

// HookInUrfaveCLI sets up an app.Before that automatically traces command runs
// and automatically updates itself.
//nolint:funlen // Why: Also not worth doing at the moment, we split a lot of this out already.
func HookInUrfaveCLI(ctx context.Context, cancel context.CancelFunc, a *cli.App, logger logrus.FieldLogger, honeycombAPIKey, dataset string) {
	env.ApplyOverrides()
	app.SetName(a.Name)

	// Ensure that we don't use the standard outreach logger
	log.SetOutput(io.Discard)

	// IDEA: Can we ever hook up --debug to this?
	overrideConfigLoaders(honeycombAPIKey, dataset, false)

	urfaveRegisterShutdownHandler(cancel)
	ctx = setupTracer(ctx, a.Name)

	exitCode, exit, cleanup := setupExitHandler(ctx)
	defer exit()

	cli.OsExiter = func(code int) { (*exitCode) = code }

	// Print a stack trace when a panic occurs and set the exit code
	defer setupPanicHandler(exitCode)

	ctx = trace.StartCall(ctx, "main")
	defer trace.EndCall(ctx)

	oldBefore := (*a).Before //nolint:gocritic // Why: we're saving the previous value
	a.Before = func(c *cli.Context) error {
		if oldBefore != nil {
			if err := oldBefore(c); err != nil {
				return err
			}
		}

		return urfaveBefore(a, logger, exit, cleanup, exitCode)(c)
	}

	// append the standard flags
	a.Flags = append(a.Flags, []cli.Flag{
		&cli.BoolFlag{
			Name:  "skip-update",
			Usage: "skips the updater check",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "enables debug logging for all components (i.e updater)",
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

	if err := a.RunContext(ctx, os.Args); err != nil {
		logger.Errorf("failed to run: %v", err)
		//nolint:errcheck // Why: We're attaching the error to the trace.
		trace.SetCallStatus(ctx, err)
		(*exitCode) = 1

		return
	}
}

// urfaveBefore is a cli.BeforeFunc that implements tracing and automatic updating
//nolint:funlen // Why: Not worth splitting out yet. May want to do so w/ more CLI support.
func urfaveBefore(a *cli.App, logger logrus.FieldLogger, exit func(), cleanup *func(),
	exitCode *int) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		cargs := c.Args().Slice()
		command := ""
		args := make([]string, 0)
		if len(cargs) > 0 {
			command = cargs[0]
		}
		if len(cargs) > 1 {
			args = cargs[1:]
		}

		userName := "unknown"
		if u, err := user.Current(); err == nil {
			userName = u.Username
		}
		trace.AddInfo(c.Context, log.F{
			a.Name + ".subcommand": command,
			a.Name + ".args":       strings.Join(args, " "),
			"os.user":              userName,
			"os.name":              runtime.GOOS,
		})

		// restart when updated
		traceCtx := trace.StartCall(c.Context, "updater.NeedsUpdate")
		defer trace.EndCall(traceCtx)

		// restart when updated
		if updater.NeedsUpdate(traceCtx, logger, "", app.Version,
			c.Bool("skip-update"), c.Bool("debug"), c.Bool("enable-prereleases"),
			c.Bool("force-update-check")) {
			switch runtime.GOOS {
			case "linux", "darwin":
				(*cleanup) = func() {
					binPath, err := exec.ResolveExecuable(os.Args[0])
					if err != nil {
						logger.WithError(err).Warn("Failed to find binary location, please re-run your command manually")
						return
					}

					logger.Infof("%s has been updated, re-running automatically", a.Name)

					//nolint:gosec // Why: We're passing in os.Args
					if err := syscall.Exec(binPath, os.Args, os.Environ()); err != nil {
						logger.WithError(err).Warn("failed to re-run binary, please re-run your command manually")
						return
					}
				}
			default:
				logger.Infof("%s has been updated, please re-run your command", a.Name)
			}

			trace.EndCall(traceCtx)

			(*exitCode) = UpdateExitCode
			trace.EndCall(traceCtx)
			exit()
			return nil
		}

		return nil
	}
}
