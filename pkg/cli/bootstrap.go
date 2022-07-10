// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains cli functions used in bootstrap
// and eventually in stencil.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/secrets"
	"github.com/getoutreach/gobox/pkg/telefork"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/updater"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/attribute"
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
			traceConfig.Otel = trace.Otel{
				Enabled:       traceConfig.Honeycomb.Enabled,
				Endpoint:      "api.honeycomb.io",
				Dataset:       traceConfig.Honeycomb.Dataset,
				SamplePercent: traceConfig.Honeycomb.SamplePercent,
				Debug:         tracingDebug,
				Stdout:        false,
				APIKey:        traceConfig.Honeycomb.APIKey,
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

func commonProps() log.Marshaler {
	commonProps := log.F{
		"os.name": runtime.GOOS,
		"os.arch": runtime.GOARCH,
	}
	if b, err := osexec.Command("git", "config", "user.email").Output(); err == nil {
		email := strings.TrimSuffix(string(b), "\n")

		// TODO: Turn the check into an config option
		// In case of @outreach.io email, we want to add PII for easier debugging with devs
		if strings.HasSuffix(email, "@outreach.io") {
			commonProps["dev.email"] = email

			if u, err := user.Current(); err == nil {
				commonProps["os.user"] = u.Username
			}

			if hostname, err := os.Hostname(); err == nil {
				commonProps["os.hostname"] = hostname
			}
			path, err := os.Getwd()
			if err == nil {
				commonProps["os.workDir"] = path
			}
		}
	}

	return commonProps
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
func setupExitHandler(ctx context.Context) (exitCode *int, exit func()) {
	exitCode = intPtr(0)
	exit = func() {
		trace.End(ctx)
		trace.CloseTracer(ctx)
		os.Exit(*exitCode)
	}

	return
}

// HookInUrfaveCLI sets up an app.Before that automatically traces command runs
// and automatically updates itself.
//nolint:funlen // Why: Also not worth doing at the moment, we split a lot of this out already.
func HookInUrfaveCLI(ctx context.Context, cancel context.CancelFunc, a *cli.App,
	logger logrus.FieldLogger, honeycombAPIKey, dataset, teleforkAPIKey string) {
	env.ApplyOverrides()
	app.SetName(a.Name)

	// Ensure that we don't use the standard outreach logger
	log.SetOutput(io.Discard)

	// IDEA: Can we ever hook up --debug to this?
	overrideConfigLoaders(honeycombAPIKey, dataset, false)

	urfaveRegisterShutdownHandler(cancel)

	c := telefork.NewClient(a.Name, teleforkAPIKey)

	trace.SetSpanProcessorHook(func(e []attribute.KeyValue) {
		c.SendEvent(e)
	})

	props := commonProps()
	c.AddInfo(props)

	ctx = setupTracer(ctx, a.Name)
	trace.AddInfo(ctx, props)

	exitCode, exit := setupExitHandler(ctx)
	defer func() {
		c.Close()
		exit()
	}()

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
		return urfaveBefore(a)(c)
	}

	if err := a.RunContext(ctx, os.Args); err != nil {
		logger.Errorf("failed to run: %v", err)
		//nolint:errcheck // Why: We're attaching the error to the trace.
		trace.SetCallStatus(ctx, err)
		(*exitCode) = 1

		return
	}
}

// urfaveBefore is a cli.BeforeFunc that implements tracing
func urfaveBefore(a *cli.App) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		trace.AddInfo(c.Context, log.F{
			"cli.subcommand": c.Args().First(),
			"cli.args":       strings.Join(c.Args().Tail(), " "),
			"os.name":        runtime.GOOS,
			"os.arch":        runtime.GOARCH,
		})
		return nil
	}
}
