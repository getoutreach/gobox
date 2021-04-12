package tracetest

import (
	"context"

	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
	"github.com/getoutreach/gobox/pkg/trace"
)

type TraceLog struct {
	events    []map[string]interface{}
	cleanupHc func()
}

func NewTraceLog() *TraceLog {
	tl := &TraceLog{}
	trace.SetTestPresendHook(tl.hcPresendHook)

	restoreSecrets := secretstest.Fake("/etc/.honeycomb_api_key", "some fake value")
	restoreConfig := env.FakeTestConfig("trace.yaml", map[string]interface{}{
		"Honeycomb": map[string]interface{}{
			"SamplePercent": 100.0,
			"APIHost":       "localhost",
			"Enabled":       true,
			"APIKey":        map[string]string{"Path": "/etc/.honeycomb_api_key"},
		},
	})

	_ = trace.StartTracing("log-testing") // nolint: errcheck

	tl.cleanupHc = func() {
		trace.CloseTracer(context.Background())
		restoreSecrets()
		restoreConfig()
		trace.SetTestPresendHook(nil)
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
