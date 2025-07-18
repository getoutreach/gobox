package tracetest

import (
	"context"
	"fmt"
	"time"

	clean "github.com/getoutreach/gobox/pkg/cleanup"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
	"github.com/getoutreach/gobox/pkg/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gotest.tools/v3/assert"
)

type Options struct {
	SamplePercent     float32
	DevEmail          string
	LogCallsByDefault bool
}

type SpanRecorder struct {
	Recorder *tracetest.SpanRecorder
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
			"Endpoint":      "blackhole.test",
			"Enabled":       true,
			"APIKey":        map[string]string{"Path": "/etc/.honeycomb_api_key"},
		},
		"LogCallsByDefault": options.LogCallsByDefault,
	}

	if options.DevEmail != "" {
		fakeConfig["GlobalTags"] = map[string]interface{}{
			"DevEmail": "test@test.com",
		}
	}

	restoreConfig, err := env.FakeTestConfigWithError("trace.yaml", fakeConfig)
	assert.NilError(nil, err)

	ctx := context.Background()
	name := "log-testing"
	_ = trace.InitTracer(ctx, name) // nolint: errcheck

	sr.Recorder = tracetest.NewSpanRecorder()
	trace.RegisterSpanProcessor(sr.Recorder)

	sr.cleanup = func() {
		// We know that CloseTracer will timeout waiting to flush pending
		// spans because we've routed it to a non-existent host and it's
		// going to be in an error backoff forever.  We set a short context
		// to make failure faster and speed up our tests.
		fastCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
		defer cancel()
		trace.CloseTracer(fastCtx)

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

	if sr.Recorder != nil {
		ended = sr.Recorder.Ended()
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

			switch a.Value.Type() {
			case attribute.INVALID:
				spanInfo[key] = nil
			case attribute.BOOL:
				spanInfo[key] = a.Value.AsBool()
			case attribute.INT64:
				spanInfo[key] = a.Value.AsInt64()
			case attribute.FLOAT64:
				spanInfo[key] = a.Value.AsFloat64()
			case attribute.STRING:
				spanInfo[key] = a.Value.AsString()
			case attribute.BOOLSLICE:
				spanInfo[key] = a.Value.AsBoolSlice()
			case attribute.INT64SLICE:
				spanInfo[key] = a.Value.AsInt64Slice()
			case attribute.FLOAT64SLICE:
				spanInfo[key] = a.Value.AsFloat64Slice()
			case attribute.STRINGSLICE:
				spanInfo[key] = a.Value.AsStringSlice()
			}
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
	cleanupCfg, err := env.FakeTestConfigWithError("trace.yaml", map[string]interface{}{
		"Otel": map[string]interface{}{
			"Enabled": false,
		},
	})

	assert.NilError(nil, err)

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
