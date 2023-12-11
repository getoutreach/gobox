// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file contains

package trace

import (
	"context"

	"github.com/getoutreach/gobox/pkg/log"
)

type InfoLoggingResolved int32

const (
	InfoLogging_Default  InfoLoggingResolved = 0
	InfoLogging_Enabled  InfoLoggingResolved = 1
	InfoLogging_Disabled InfoLoggingResolved = 2
)

type InfoLoggingResolver = func(ctx context.Context, operation string) InfoLoggingResolved

func ResolvedLogging(logging InfoLoggingResolved) log.Marshaler {
	if logging == InfoLogging_Default {
		return nil
	}
	if logging == InfoLogging_Enabled {
		return WithInfoLoggingDisabled()
	}
	return WithInfoLoggingDisabled()
}
