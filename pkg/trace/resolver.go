// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file contains constants and tools for controlling trace.Start/EndCall logging

package trace

import (
	"context"

	"github.com/getoutreach/gobox/pkg/log"
)

type InfoLoggingResolved int32

const (
	// InfoLoggingDefault leaves the configured default in place.
	InfoLoggingDefault InfoLoggingResolved = 0

	// InfoLoggingEnabled emits the info log for this call.
	InfoLoggingEnabled InfoLoggingResolved = 1

	// InfoLoggingDisabled suppresses the info log for this call; LogTracedCalls
	// will not re-enable it.
	InfoLoggingDisabled InfoLoggingResolved = 2

	// InfoLoggingSampledOut suppresses the info log like InfoLoggingDisabled, but
	// LogTracedCalls may still promote it.  Use it for rate-based sampling.
	InfoLoggingSampledOut InfoLoggingResolved = 3
)

type InfoLoggingResolver = func(ctx context.Context, operation string) InfoLoggingResolved

// ResolvedLogging returns signals trace.EndCall whether to enable/disable info logging
func ResolvedLogging(logging InfoLoggingResolved) log.Marshaler {
	switch logging {
	case InfoLoggingDefault:
		return nil
	case InfoLoggingEnabled:
		return WithInfoLoggingEnabled()
	case InfoLoggingSampledOut:
		return withSampledOutInfoLogging()
	case InfoLoggingDisabled:
		return WithInfoLoggingDisabled()
	}

	// Anything unrecognized suppresses the log.
	return WithInfoLoggingDisabled()
}

// ReevaluateLogging re-runs the resolver for the current call.  A result of
// InfoLoggingDefault leaves the current decision alone.
func ReevaluateLogging(ctx context.Context, resolver InfoLoggingResolver) {
	logging := resolver(ctx, GetCallName(ctx))
	if logging == InfoLoggingDefault {
		return
	}

	callInfo := callTracker.Info(ctx)
	callInfo.Opts.EnableInfoLogging = logging == InfoLoggingEnabled

	// Sampled out stays eligible for LogTracedCalls; every other answer is final.
	callInfo.InfoLoggingExplicit = logging != InfoLoggingSampledOut

	logTracedCall(ctx)
}
