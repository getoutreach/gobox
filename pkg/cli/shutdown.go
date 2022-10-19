// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains shutdown related code for CLIs.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/getoutreach/gobox/pkg/trace"
)

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

// setupPanicHandler sets up a panic handler that will print the panic message
// and stack trace to stderr, and then set the exit code to 2.
func setupPanicHandler(exitCode *int) {
	if r := recover(); r != nil {
		fmt.Printf("stacktrace from panic: %s\n%s\n", r, string(debug.Stack()))

		// Go sets panic exit codes to 2
		(*exitCode) = 2
	}
}

// setupExitHandler sets up an exit handler that will call os.Exit() with
// the set exit code, ensuring that all log/trace data is flushed.
func setupExitHandler() (exitCode *int, exit func()) {
	exitCodeInt := 0
	exitCode = &exitCodeInt
	// exit runs all shutdown hooks and then calls os.Exit with the exit code
	exit = func() {
		trace.ForceFlush(context.Background())
		os.Exit(*exitCode)
	}
	return
}
