// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: Implements a compatible API for the olog package and log package.

package log

import (
	"context"
	"io"
	"log/slog"

	"github.com/getoutreach/gobox/pkg/olog"
	"github.com/sirupsen/logrus"
)

// Logger is the compatible log interface
type Logger struct {
	logrusLogger *logrus.Logger
	ologLogger   *slog.Logger
	useLogrus    bool
}

func New(useLogrus bool) *Logger {
	if useLogrus {
		l := logrus.New()
		return &Logger{logrusLogger: l, useLogrus: true}
	}
	l := olog.New()
	return &Logger{ologLogger: l, useLogrus: false}
}

func (log *Logger) Debug(ctx context.Context, message string, m Marshaler) {
	if log.useLogrus {
		Debug(ctx, message, m)
	} else {
		// add data field to log
		log.ologLogger.DebugContext(ctx, message, "attr", m)
	}
}

func (log *Logger) Info(ctx context.Context, message string, m Marshaler) {
	if log.useLogrus {
		Info(ctx, message, m)
	} else {
		log.ologLogger.InfoContext(ctx, message, "attr", m)
	}
}

func (log *Logger) Warn(ctx context.Context, message string, m Marshaler) {
	if log.useLogrus {
		Warn(ctx, message, m)
	} else {
		log.ologLogger.WarnContext(ctx, message, "attr", m)
	}
}

func (log *Logger) Error(ctx context.Context, message string, m Marshaler) {
	if log.useLogrus {
		Error(ctx, message, m)
	} else {
		log.ologLogger.ErrorContext(ctx, message, "attr", m)
	}
}

func (log *Logger) Fatal(ctx context.Context, message string, m Marshaler) {
	if log.useLogrus {
		Fatal(ctx, message, m)
	} else {
		// slog does not have Fatal level, use Error level
		log.ologLogger.ErrorContext(ctx, message, "attr", m)
	}
}

func (log *Logger) SetOutput(w io.Writer) {
	if log.useLogrus {
		SetOutput(w)
	} else {
		olog.SetOutput(w)
		// recreate the logger with new output
		log.ologLogger = olog.New()
	}
}
