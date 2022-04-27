package tracetest

import (
	"context"

	clean "github.com/getoutreach/gobox/pkg/cleanup"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
	"github.com/getoutreach/gobox/pkg/trace"
)

type TraceLog struct {
	events    []map[string]interface{}
	cleanupHc func()
}

type Options struct {
	SamplePercent float32
}

func NewTraceLog() *TraceLog {
	return NewTraceLogWithOptions(
		Options{
			SamplePercent: 100.0,
		},
	)
}
func NewTraceLogWithOptions(options Options) *TraceLog {
	tl := &TraceLog{}
	trace.SetPresendHook(tl.hcPresendHook)

	restoreSecrets := secretstest.Fake("/etc/.honeycomb_api_key", "some fake value")
	restoreConfig := env.FakeTestConfig("trace.yaml", map[string]interface{}{
		"Honeycomb": map[string]interface{}{
			"SamplePercent": options.SamplePercent,
			"APIHost":       "localhost",
			"Enabled":       true,
			"APIKey":        map[string]string{"Path": "/etc/.honeycomb_api_key"},
		},
	})

	ctx := context.Background()
	_ = trace.InitTracer(ctx, "log-testing") // nolint: errcheck

	tl.cleanupHc = func() {
		trace.CloseTracer(ctx)
		restoreSecrets()
		restoreConfig()
		trace.SetPresendHook(nil)
	}

	return tl
}

func (tl *TraceLog) hcPresendHook(event map[string]interface{}) {
	tl.events = append(tl.events, event)
}

func (tl *TraceLog) HoneycombEvents() []map[string]interface{} {
	return tl.events
}

func (tl *TraceLog) Close() {
	tl.cleanupHc()
}

// Disabled method disables the tracing test-infra and return cleanup function to be called after test finished.
// The cleanup function resets the tracing secrets and configuration.
func Disabled() (cleanup func()) {
	cleanupSecrets := secretstest.Fake("/etc/.honeycomb_api_key", "some fake value")
	cleanupCfg := env.FakeTestConfig("trace.yaml", map[string]interface{}{
		"Honeycomb": map[string]interface{}{
			"Enabled": false,
		},
	})
	if err := trace.InitTracer(context.Background(), "log-testing"); err != nil {
		panic(err.Error())
	}

	cleanupAll := clean.Funcs{&cleanupSecrets, &cleanupCfg}
	return cleanupAll.All()
}

// Disable methods disables the tracing.
func Disable() {
	Disabled()()
}
