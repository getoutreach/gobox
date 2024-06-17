// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file contains constants and tools for controlling trace.Start/EndCall logging

package trace

import (
	"context"

	"github.com/getoutreach/gobox/pkg/log"
)

type InfoLoggingResolved int32

const (
	InfoLoggingDefault  InfoLoggingResolved = 0
	InfoLoggingEnabled  InfoLoggingResolved = 1
	InfoLoggingDisabled InfoLoggingResolved = 2
)

type InfoLoggingResolver = func(ctx context.Context, operation string) InfoLoggingResolved

// ResolvedLogging returns signals trace.EndCall whether to enable/disable info logging
func ResolvedLogging(logging InfoLoggingResolved) log.Marshaler {
	if logging == InfoLoggingDefault {
		return nil
	}
	if logging == InfoLoggingEnabled {
		return WithInfoLoggingEnabled()
	}
	return WithInfoLoggingDisabled()
}

func ReevaluateLogging(ctx context.Context, resolver InfoLoggingResolver) {
	logging := resolver(ctx, GetCallName(ctx))
	if logging == InfoLoggingDefault {
		return
	}
	callInfo := callTracker.Info(ctx)
	callInfo.Opts.EnableInfoLogging = logging == InfoLoggingEnabled
}
