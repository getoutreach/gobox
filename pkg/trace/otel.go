package trace

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

type otelTracer struct {
	Config
	sync.Once
	serviceName    string
	tracerProvider *sdktrace.TracerProvider
	forceTrace     bool
	forceNoTrace   bool
}

// Annotator is a SpanProcessor that adds service-level tags on every span
type Annotator struct {
	globalTags GlobalTags
	sampleRate int64
}

func (a Annotator) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	setf := func(key string, value interface{}) {
		s.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
	}

	s.SetAttributes(attribute.Int64("SampleRate", a.sampleRate))
	logf.Marshal("", app.Info(), setf)
	logf.Marshal("", a.globalTags, setf)
}

func (a Annotator) OnEnd(s sdktrace.ReadOnlySpan) {
	if spanProcessorHook != nil {
		spanProcessorHook(s.Attributes())
	}
}

func (a Annotator) Shutdown(context.Context) error { return nil }

func (a Annotator) ForceFlush(context.Context) error { return nil }

// nolint:gochecknoglobals
var spanProcessorHook func([]attribute.KeyValue)

// SetSpanProcessorHook sets a hook to run when a span ends
func SetSpanProcessorHook(hook func([]attribute.KeyValue)) {
	spanProcessorHook = hook
}

// Deprecated: Use initTracer() instead.
func (t *otelTracer) startTracing(serviceName string) error {
	return t.initTracer(context.TODO(), serviceName)
}

func (t *otelTracer) registerSpanProcessor(s sdktrace.SpanProcessor) {
	t.tracerProvider.RegisterSpanProcessor(s)
}

func (t *otelTracer) initTracer(ctx context.Context, serviceName string) error {
	key, err := t.Otel.APIKey.Data(ctx)
	if err != nil {
		log.Error(ctx, "Unable to fetch otel API key", events.NewErrorInfo(err))
	}

	headers := map[string]string{
		"x-honeycomb-team":    strings.TrimSpace(string(key)),
		"x-honeycomb-dataset": t.Otel.Dataset,
	}

	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(t.Otel.Endpoint),
		otlptracehttp.WithHeaders(headers),
	)

	exp, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Error(ctx, "Unable to start trace exporter", events.NewErrorInfo(err))
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		log.Error(ctx, "Unable to configure trace provider", events.NewErrorInfo(err))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
		// accepts sample rates as number of requests seen per request sampled
		sdktrace.WithSampler(forceSample(uint(100/t.Otel.SamplePercent))),
		sdktrace.WithSpanProcessor(Annotator{
			globalTags: t.GlobalTags,
			sampleRate: int64(100 / t.Otel.SamplePercent),
		}),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	t.serviceName = serviceName
	t.tracerProvider = tp

	t.tracerProvider.Tracer(serviceName)

	return nil
}

// Deprecated: Use closeTracer() instead.
func (t *otelTracer) endTracing() {
	t.closeTracer(context.TODO())
}

func (t *otelTracer) closeTracer(ctx context.Context) {
	if t.tracerProvider == nil {
		return
	}

	t.tracerProvider.ForceFlush(ctx)
	err := t.tracerProvider.Shutdown(ctx)
	if err != nil {
		log.Error(ctx, "Unable to stop otel tracer", events.NewErrorInfo(err))
	}
}

func (t *otelTracer) startTrace(ctx context.Context, name string) context.Context {
	tracer := otel.GetTracerProvider().Tracer(t.serviceName)
	ctx, _ = tracer.Start(ctx, name)

	return ctx
}

func (t *otelTracer) id(ctx context.Context) string {
	if span := trace.SpanFromContext(ctx); span != nil && span.SpanContext().TraceID().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

func (t *otelTracer) startSpan(ctx context.Context, name string) context.Context {
	tracer := otel.GetTracerProvider().Tracer(t.serviceName)
	ctx, _ = tracer.Start(ctx, name)

	return ctx
}

func (t *otelTracer) startSpanAsync(ctx context.Context, name string) context.Context {
	tracer := otel.GetTracerProvider().Tracer(t.serviceName)
	ctx, _ = tracer.Start(ctx, name)

	return ctx
}

func (t *otelTracer) end(ctx context.Context) {
	if span := trace.SpanFromContext(ctx); span != nil {
		span.End()
	}
}

func (t *otelTracer) addInfo(ctx context.Context, args ...log.Marshaler) {
	if span := trace.SpanFromContext(ctx); span != nil {
		for _, f := range args {
			logf.Marshal("", f, func(key string, value interface{}) {
				span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
			})
		}
	}
}

func (t *otelTracer) spanID(ctx context.Context) string {
	if span := trace.SpanFromContext(ctx); span != nil && span.SpanContext().SpanID().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// Deprecated: Will be removed with full migration to OpenTelemetry.
// OpenTelemetry automatically handle adding parentID to traces
func (t *otelTracer) parentID(ctx context.Context) string {
	return ""
}

func (t *otelTracer) setForce(force bool) {
	t.forceTrace = force
}

func (t *otelTracer) isForce() bool {
	return t.forceTrace
}

func (t *otelTracer) setForceNoTrace(ctx context.Context, forceNoTrace bool) context.Context {
	t.forceNoTrace = forceNoTrace

	if t.forceNoTrace {
		if span := trace.SpanFromContext(ctx); span != nil {
			ctx = trace.ContextWithSpan(ctx, nil)
		}
	}

	return ctx
}
