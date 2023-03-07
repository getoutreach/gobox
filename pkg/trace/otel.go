// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements the trace interface for OpenTelemetry based tracing

package trace

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
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
	force          bool
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
	app.Info().MarshalLog(setf)
	a.globalTags.MarshalLog(setf)
}

func (a Annotator) OnEnd(s sdktrace.ReadOnlySpan) {
	if spanProcessorHook != nil {
		spanProcessorHook(s.Attributes())
	}
}

func (a Annotator) Shutdown(context.Context) error { return nil }

func (a Annotator) ForceFlush(context.Context) error { return nil }

// nolint:gochecknoglobals // Why: need to allow overriding
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
	mp := metric.NewNoopMeterProvider()
	global.SetMeterProvider(mp)

	key, err := t.Otel.APIKey.Data(ctx)
	if err != nil {
		log.Error(ctx, "Unable to fetch otel API key", events.NewErrorInfo(err))
	}

	headers := map[string]string{
		"x-honeycomb-team":    strings.TrimSpace(string(key)),
		"x-honeycomb-dataset": t.Otel.Dataset,
	}

	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(t.Otel.Endpoint+":443"),
		otlptracegrpc.WithHeaders(headers),
	)

	exp, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Error(ctx, "Unable to start trace exporter", events.NewErrorInfo(err))
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		log.Error(ctx, "Unable to configure trace provider", events.NewErrorInfo(err))
	}

	tpOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
		// accepts sample rates as number of requests seen per request sampled
		sdktrace.WithSampler(forceSample(uint(100 / t.Otel.SamplePercent))),
		sdktrace.WithSpanProcessor(Annotator{
			globalTags: t.GlobalTags,
			sampleRate: int64(100 / t.Otel.SamplePercent),
		}),
	}

	if t.Otel.CollectorEndpoint != "" {
		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(t.Otel.CollectorEndpoint),
		)

		exp, err := otlptrace.New(ctx, client)
		if err != nil {
			log.Error(ctx, "Unable to start open telemetry collector trace exporter", events.NewErrorInfo(err))
		}

		tpOptions = append(tpOptions, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(tpOptions...)

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

func (t *otelTracer) startSpan(ctx context.Context, name string, opts ...SpanStartOption) context.Context {
	tracer := otel.GetTracerProvider().Tracer(t.serviceName)
	ctx, _ = tracer.Start(ctx, name, t.toOtelOptions(opts)...)

	return ctx
}

func (t *otelTracer) toOtelOptions(opts []SpanStartOption) []trace.SpanStartOption {
	otelOpts := []trace.SpanStartOption{}
	for _, opt := range opts {
		otelOpt := opt.otelOption()
		if otelOpt != nil {
			otelOpts = append(otelOpts, otelOpt)
		}
	}
	return otelOpts
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
		for _, arg := range args {
			kvs := marshalToKeyValue(arg)
			span.SetAttributes(kvs...)

			switch v := arg.(type) {
			case *events.ErrorInfo:
				// In this case we use the raw error-- the other attributes of
				// *events.ErrorInfo will be sent along via the above call to
				// span.SetAttributes
				setError(span, v.RawError)
			case error:
				// Any log.Marshaler could also implement error, in which case we want
				// to respect that the client intended to send an error, and set the
				// appropriate attributes on the span
				setError(span, v)
			default:
				// do nothing
			}
		}
	}
}

// setError sets the error code and sends an error event. setting the code
// will cause error spans to be called out specifically in the trace, and
// will make the "errors" default view in honeycomb useful
func setError(span trace.Span, err error) {
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}

// nolint:gocyclo // Why: It's a big case statement that's hard to split.
func marshalToKeyValue(arg log.Marshaler) []attribute.KeyValue {
	res := []attribute.KeyValue{}

	logf.Marshal("", arg, func(key string, value interface{}) {
		switch v := value.(type) {
		case []bool:
			res = append(res, attribute.BoolSlice(key, v))
		case []int:
			res = append(res, attribute.IntSlice(key, v))
		case []int8:
			int64s := make([]int64, len(v))
			for i, elem := range v {
				int64s[i] = int64(elem)
			}
			res = append(res, attribute.Int64Slice(key, int64s))
		case []int16:
			int64s := make([]int64, len(v))
			for i, elem := range v {
				int64s[i] = int64(elem)
			}
			res = append(res, attribute.Int64Slice(key, int64s))
		case []int32:
			int64s := make([]int64, len(v))
			for i, elem := range v {
				int64s[i] = int64(elem)
			}
			res = append(res, attribute.Int64Slice(key, int64s))
		case []int64:
			res = append(res, attribute.Int64Slice(key, v))
		case []uint8:
			int64s := make([]int64, len(v))
			for i, elem := range v {
				int64s[i] = int64(elem)
			}
			res = append(res, attribute.Int64Slice(key, int64s))
		case []uint16:
			int64s := make([]int64, len(v))
			for i, elem := range v {
				int64s[i] = int64(elem)
			}
			res = append(res, attribute.Int64Slice(key, int64s))
		case []uint32:
			int64s := make([]int64, len(v))
			for i, elem := range v {
				int64s[i] = int64(elem)
			}
			res = append(res, attribute.Int64Slice(key, int64s))
		// []uint and []uint64 aren't safe to cast.  We stringify them.
		case []uint:
			strs := make([]string, len(v))
			for i, elem := range v {
				strs[i] = fmt.Sprintf("%d", elem)
			}
			res = append(res, attribute.StringSlice(key, strs))
		case []uint64:
			strs := make([]string, len(v))
			for i, elem := range v {
				strs[i] = fmt.Sprintf("%d", elem)
			}
			res = append(res, attribute.StringSlice(key, strs))
		case []float32:
			float64s := make([]float64, len(v))
			for i, elem := range v {
				float64s[i] = float64(elem)
			}
			res = append(res, attribute.Float64Slice(key, float64s))
		case []float64:
			res = append(res, attribute.Float64Slice(key, v))
		case []string:
			res = append(res, attribute.StringSlice(key, v))
		case bool:
			res = append(res, attribute.Bool(key, v))
		case int:
			res = append(res, attribute.Int(key, v))
		case int8:
			res = append(res, attribute.Int64(key, int64(v)))
		case int16:
			res = append(res, attribute.Int64(key, int64(v)))
		case int32:
			res = append(res, attribute.Int64(key, int64(v)))
		case int64:
			res = append(res, attribute.Int64(key, v))
		case uint8:
			res = append(res, attribute.Int64(key, int64(v)))
		case uint16:
			res = append(res, attribute.Int64(key, int64(v)))
		case uint32:
			res = append(res, attribute.Int64(key, int64(v)))
			// We can't guarantee that uint64 or uint can be safely casted
			// to int64.  We let them fall through to be strings.  :/
		case float32:
			res = append(res, attribute.Float64(key, float64(v)))
		case float64:
			res = append(res, attribute.Float64(key, v))
		case string:
			res = append(res, attribute.String(key, v))
		case time.Time:
			// This is a compromise.  OTel seems to
			// prefer UNIX epoch milliseconds, while
			// Honeycomb says it accepts UNIX epoch
			// seconds.  Honeycomb also has a function to
			// convert RFC3339 timestamps to epoch.
			//
			// We figure RFC3339 is unambiguously a
			// timestamp and expect most systems can
			// deal with it accordingly.  Magic ints
			// or floats without units attached would
			// be harder to interpret.
			res = append(res, attribute.String(key, v.Format(time.RFC3339Nano)))
		default:
			res = append(res, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	})

	return res
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
	t.force = force
}

func (t *otelTracer) isForce() bool {
	return t.force
}
