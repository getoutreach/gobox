package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/olog"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var TestLogrus = logrus.New()
var TestOlogger = olog.New()

func TestNewLoggerWithLogrus(t *testing.T) {
	ctx := context.Background()
	logger := log.New(true)
	f := log.F{"app": "logger_test"}
	assert.IsType(t, &logrus.Logger{}, TestLogrus)
	logger.Info(ctx, "this is a test", f)
}

func TestNewLoggerWithOlog(t *testing.T) {
	ctx := context.Background()
	logger := log.New(false)
	f := log.F{"app": "logger_test"}
	assert.IsType(t, &slog.Logger{}, TestOlogger)
	logger.Info(ctx, "this is a test", f)
}

func TestLogrusFunction(t *testing.T) {
	ctx := context.Background()
	logger := log.New(true)
	f := log.F{"app": "logger_test"}

	var b bytes.Buffer
	logger.SetOutput(&b)
	logger.Info(ctx, "info msg", f)
	assert.Contains(t, b.String(), "info msg")
	assert.Contains(t, b.String(), "\"level\":\"INFO\"")
}

func TestLogrusErrorFunction(t *testing.T) {
	ctx := context.Background()
	logger := log.New(true)
	f := log.F{"app": "logger_test"}

	var b bytes.Buffer
	logger.SetOutput(&b)
	logger.Error(ctx, "error msg", f)

	assert.Contains(t, b.String(), "error msg")
	assert.Contains(t, b.String(), "\"level\":\"ERROR\"")
}

func TestOlogFunction(t *testing.T) {
	ctx := context.Background()
	logger := log.New(false)
	f := log.F{"app": "logger_test"}
	b := new(bytes.Buffer)
	logger.SetOutput(b)

	logger.Info(ctx, "olog info msg", f)

	assert.NotEmpty(t, b)
	assert.Contains(t, b.String(), "olog info msg")
	assert.Contains(t, b.String(), "\"level\":\"INFO\"")
}

func TestOlogErrorFunction(t *testing.T) {
	ctx := context.Background()
	logger := log.New(false)
	f := log.F{"app": "logger_test"}
	b := new(bytes.Buffer)
	logger.SetOutput(b)

	logger.Error(ctx, "olog error msg", f)

	assert.NotEmpty(t, b)
	assert.Contains(t, b.String(), "olog error msg")
	assert.Contains(t, b.String(), "\"level\":\"ERROR\"")
}
