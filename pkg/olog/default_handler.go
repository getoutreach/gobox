// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains logic for determining which handler should be
// used by default.

package olog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"sync/atomic"

	charmlog "github.com/charmbracelet/log"

	"golang.org/x/term"
)

var (
	// defaultHandler is the global handler used by all loggers returned
	// by this package. For details on how this is set, see
	// `determineDefaultHandler`.
	defaultHandler atomic.Int32

	// defaultOut is the default output for the default handler. This is
	// set to `os.Stderr` by default.
	defaultOut io.Writer = os.Stderr

	// mainModule is the module that the current binary was built in.
	mainModule = debug.Module{}
)

// DefaultHandlerType denotes which handler should be used by default.
// This is calculated via the `setDefaultHandler` function on package
// init.
type DefaultHandlerType int

const (
	JSONHandler DefaultHandlerType = iota
	TextHandler
)

// determineMainModule uses the `debug.ReadBuildInfo` function to
// determine the module that the current binary was built in.
func determineMainModule() {
	m, ok := debug.ReadBuildInfo()
	if !ok {
		//nolint:errorlint // Why: We can't wrap panic-d errors.
		panic(fmt.Errorf("failed to read build info (must be built with GO111MODULE=on)"))
	}

	mainModule = m.Main
}

// determineDefaultHandler sets the default handler based on the current
// environment. If the `defaultOut` is a TTY, then the default handler
// is a `slog.TextHandler`. Otherwise, the default handler is the
// `slog.JSONHandler`.
func determineDefaultHandler() {
	out, ok := defaultOut.(*os.File)
	if !ok {
		// If the default output is not a file, then we can't
		// determine if it's a TTY or not. So, we default to JSON.
		defaultHandler.Store(int32(JSONHandler))
		return
	}

	if term.IsTerminal(int(out.Fd())) {
		defaultHandler.Store(int32(TextHandler))
	} else {
		defaultHandler.Store(int32(JSONHandler))
	}
}

// init sets the default handler and determines the main (parent) module
// of the current program. See `determineDefaultHandler` and
// `detemineMainModule` for more information.
//
//nolint:gochecknoinits // Why: Initializes the default handler.
func init() {
	determineDefaultHandler()
	determineMainModule()
}

// SetDefaultHandler changes the default handler to be the provided
// type. This must be called before any loggers are created to have an
// effect on all loggers.
func SetDefaultHandler(ht DefaultHandlerType) {
	defaultHandler.Store(int32(ht))
}

// createHandler creates a new handler for usage with a slog.Logger. The
// handler used is determined based on the current defaultHandler. The
// handler is configured to add source information to all logs as well
// as determine the log level with the `leveler` implementation provided
// by this package. The `leveler` implementation is configured to use
// the provided moduleName and packageName as addresses for determining
// the log level.
//
// `lr` should be `globalLevelRegistry` unless you need to change the
// log level for tests (in the olog package).
func createHandler(lr *levelRegistry, m *metadata) slog.Handler {
	var h slog.Handler
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level: newLeveler(lr, []string{
			// Order is important here, the first address that
			// matches will be used. So, we start with the most granular
			// address, the package name.
			m.PackagePath,
			m.ModulePath,
		}),
		ReplaceAttr: replaceKey("time", "@timestamp"),
	}

	switch DefaultHandlerType(defaultHandler.Load()) {
	case JSONHandler:
		h = slog.NewJSONHandler(defaultOut, opts)
	case TextHandler:
		// TODO(jaredallard): There's no support for slog.Leveler in the
		// current charmbracelet/log implementation. So, we can't
		// dynamically change the logging level yet.
		//
		// https://github.com/charmbracelet/log/issues/98
		var charmLogLevel charmlog.Level
		switch opts.Level.Level() {
		case slog.LevelDebug:
			charmLogLevel = charmlog.DebugLevel
		case slog.LevelInfo:
			charmLogLevel = charmlog.InfoLevel
		case slog.LevelWarn:
			charmLogLevel = charmlog.WarnLevel
		case slog.LevelError:
			charmLogLevel = charmlog.ErrorLevel
		default:
			panic("unknown slog level")
		}

		h = charmlog.NewWithOptions(defaultOut, charmlog.Options{
			ReportTimestamp: true,
			TimeFormat:      "15:04:05",
			ReportCaller:    opts.AddSource,
			Level:           charmLogLevel,
		})
	default:
		panic("unknown default handler")
	}

	// When running in the main module, we don't need to add any extra
	// keys to the handler.
	if mainModule.Path == m.ModulePath {
		return h
	}

	// Return the handler with the default keys set.
	// - module: the module that logged this message.
	// - modulever: the version of the module that logged this message.
	return h.WithAttrs([]slog.Attr{
		{Key: "module", Value: slog.StringValue(m.ModulePath)},
		{Key: "modulever", Value: slog.StringValue(m.ModuleVersion)},
	})
}

func replaceKey(oldKey, newKey string) func([]string, slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == oldKey {
			a.Key = newKey
		}
		return a
	}
}
