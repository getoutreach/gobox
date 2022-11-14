package tracetest

import (
	"context"
	"fmt"

	clean "github.com/getoutreach/gobox/pkg/cleanup"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
	"github.com/getoutreach/gobox/pkg/trace"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type Options struct {
	SamplePercent float32
	DevEmail      string
}

type SpanRecorder struct {
	recorder *tracetest.SpanRecorder
	cleanup  func()
}

func NewSpanRecorder() *SpanRecorder {
	return NewSpanRecorderWithOptions(
		Options{
			SamplePercent: 100.0,
			DevEmail:      "",
		},
	)
}

func NewSpanRecorderWithOptions(options Options) *SpanRecorder {
	sr := &SpanRecorder{}

	restoreSecrets := secretstest.Fake("/etc/.honeycomb_api_key", "some fake value")

	fakeConfig := map[string]interface{}{
		"OpenTelemetry": map[string]interface{}{
			"SamplePercent": options.SamplePercent,
			"Endpoint":      "localhost",
			"Enabled":       true,
			"APIKey":        map[string]string{"Path": "/etc/.honeycomb_api_key"},
		},
	}

	if options.DevEmail != "" {
		fakeConfig["GlobalTags"] = map[string]interface{}{
			"DevEmail": "test@test.com",
		}
	}

	restoreConfig := env.FakeTestConfig("trace.yaml", fakeConfig)

	ctx := context.Background()
	name := "log-testing"
	_ = trace.InitTracer(ctx, name) // nolint: errcheck

	sr.recorder = tracetest.NewSpanRecorder()
	trace.RegisterSpanProcessor(sr.recorder)

	sr.cleanup = func() {
		trace.CloseTracer(ctx)
		restoreSecrets()
		restoreConfig()
	}

	return sr
}

func (sr *SpanRecorder) Close() {
	sr.cleanup()
}

func (sr *SpanRecorder) Ended() []map[string]interface{} {
	var ended []oteltrace.ReadOnlySpan

	if sr.recorder != nil {
		ended = sr.recorder.Ended()
	}

	result := make([]map[string]interface{}, 0, len(ended))
	for _, s := range ended {
		spanContext := s.SpanContext()
		parent := s.Parent()

		spanInfo := map[string]interface{}{
			"name":                   s.Name(),
			"spanContext.traceID":    spanContext.TraceID().String(),
			"spanContext.spanID":     spanContext.SpanID().String(),
			"spanContext.traceFlags": spanContext.TraceFlags().String(),
			"parent.traceID":         parent.TraceID().String(),
			"parent.spanID":          parent.SpanID().String(),
			"parent.traceFlags":      parent.TraceFlags().String(),
			"parent.remote":          parent.IsRemote(),
			"spanKind":               s.SpanKind().String(),
			"startTime":              s.StartTime().String(),
			"endTime":                s.EndTime().String(),
		}

		for _, a := range s.Attributes() {
			if a.Key == "SampleRate" {
				spanInfo["SampleRate"] = a.Value.AsInt64()
				continue
			}

			key := fmt.Sprintf("attributes.%s", a.Key)
			spanInfo[key] = a.Value.AsString()
		}

		links := s.Links()
		if len(links) > 0 {
			var linksInfo []map[string]interface{}
			for _, link := range links {
				linkInfo := map[string]interface{}{
					"spanContext.traceID": link.SpanContext.TraceID().String(),
					"spanContext.spanID":  link.SpanContext.SpanID().String(),
				}
				linksInfo = append(linksInfo, linkInfo)
			}
			spanInfo["links"] = linksInfo
		}

		result = append(result, spanInfo)
	}

	return result
}

// Disabled method disables the tracing test-infra and return cleanup function to be called after test finished.
// The cleanup function resets the tracing secrets and configuration.
func Disabled() (cleanup func()) {
	cleanupSecrets := secretstest.Fake("/etc/.honeycomb_api_key", "some fake value")
	cleanupCfg := env.FakeTestConfig("trace.yaml", map[string]interface{}{
		"Otel": map[string]interface{}{
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
