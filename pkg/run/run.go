// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: Entrypoint to run the app

// Package run provides a function that can be invoked inside of main to set up
// all the service-standard components we expect every service to run.
//
// clients should provide any runners they require as part of the app to the
// Run function with OptAddRunner, and then call `Run` in their main function.
package run

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/olog"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/stencil-golang/pkg/serviceactivities/automemlimit"
	"github.com/getoutreach/stencil-golang/pkg/serviceactivities/gomaxprocs"
	"github.com/getoutreach/stencil-golang/pkg/serviceactivities/shutdown"

	"github.com/getoutreach/httpx/pkg/handlers"
)

// runOpts are the options to pass to Run to configure your service
type runOpts struct {
	httpAppHandler http.Handler
	log            *slog.Logger
	httpAddr       string
	runners        []async.Runner
}

// Option is the interface for an  option
type Option interface {
	apply(*runOpts) error
}

// optionFunc is a function that implements serviceOption
type optionFunc func(*runOpts) error

// apply implements serviceOption.
func (s optionFunc) apply(o *runOpts) error {
	return s(o)
}

// OptLogger sets the logger. Otherwise it defaults to olog.New
func OptLogger(l *slog.Logger) Option {
	return optionFunc(func(o *runOpts) error {
		o.log = l
		return nil
	})
}

// OptHTTPAppHandler sets the http handler for the app. Otherwise it defaults to NotFound
func OptHTTPAppHandler(appHandler http.Handler) Option {
	return optionFunc(func(o *runOpts) error {
		o.httpAppHandler = appHandler
		return nil
	})
}

// OptAddRunner adds a runnable the service needs to run. If the runnable
// exits, so does the service
func OptAddRunner(name string, r async.Runner) Option {
	return optionFunc(func(o *runOpts) error {
		o.runners = append(o.runners, async.Func(
			func(ctx context.Context) error {
				o.log.Info(fmt.Sprintf("starting %s", name), "runner.name", name)
				err := r.Run(ctx)
				if err != nil {
					o.log.Warn(fmt.Sprintf("exited %s with error", name), "runner.name", name, "error", err)
				}
				o.log.Info(fmt.Sprintf("exited %s", name), "runner.name", name)
				return err
			}))
		return nil
	})
}

// OptConfigLoader defines an alternative config loader
func OptHTTPAddr(addr string) Option {
	return optionFunc(func(o *runOpts) error {
		o.httpAddr = addr
		return nil
	})
}

// Run runs your service.
//
// If [ctx] ends, your app will stop.
// [name] is the name of your service
// [options] are functional options that allow you to add more runnables (e.g.
// gRPC server, queue consumers, etc.), or to configure the http server and the
// logger used in this method. See types in this package prefixed with `Opt`
func Run(
	ctx context.Context,
	name string,
	options ...Option,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// deafult options
	opts := &runOpts{
		httpAppHandler: http.NotFoundHandler(),
		log:            olog.New(),
		httpAddr:       "127.0.0.1:5000",
	}

	for _, o := range options {
		err := o.apply(opts)
		if err != nil {
			return err
		}
	}

	log := opts.log

	env.ApplyOverrides()
	app.SetName(name)

	if err := trace.InitTracer(ctx, name); err != nil {
		return fmt.Errorf("%w starting tracing", err)
	}
	defer trace.CloseTracer(ctx)

	log.InfoContext(ctx, "starting", "app", app.Info(), slog.Int("app.pid", os.Getpid()))

	httpService := handlers.Service{
		App: opts.httpAppHandler,
	}

	// always required runners
	acts := []async.Runner{
		shutdown.New(),
		gomaxprocs.New(),
		automemlimit.New(),
		async.Func(func(ctx context.Context) error {
			return httpService.Run(ctx, opts.httpAddr)
		}),
	}

	// add runners from options
	acts = append(acts, opts.runners...)

	err := async.RunGroup(acts).Run(ctx)
	if shutdown.HandleShutdownConditions(ctx, err) {
		return nil
	}
	return fmt.Errorf("%w lead to shutdown", err)
}
