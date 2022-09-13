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
	"github.com/getoutreach/gobox/pkg/cli/updater"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/secrets"
	"github.com/getoutreach/gobox/pkg/telefork"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
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
				Otel: trace.Otel{
					Enabled:  true,
					Endpoint: "api.honeycomb.io",
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
	return trace.StartSpan(ctx, name)
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
			// deubgging.
			logger.Errorf("failed to generate completion: %v", err)
		}
		return
	}

	env.ApplyOverrides()
	app.SetName(a.Name)

	// Ensure that we don't use the standard outreach logger
	log.SetOutput(io.Discard)

	// IDEA: Can we ever hook up --debug to this?
	overrideConfigLoaders(honeycombAPIKey, dataset, false)

	urfaveRegisterShutdownHandler(cancel)

	props := commonProps()

	// Create a tracer with a span initialized.
	ctx = setupTracer(ctx, a.Name)
	trace.AddInfo(ctx, props)

	// Configure tracing to talk to a telefork client.
	c := telefork.NewClient(a.Name, teleforkAPIKey)
	c.AddInfo(props)
	trace.SetSpanProcessorHook(func(e []attribute.KeyValue) {
		c.SendEvent(e)
	})
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
