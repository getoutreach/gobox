package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/getoutreach/gobox/pkg/cli/logfile"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func NewLogFileTracer(ctx context.Context, serviceName string, config Config) (tracer, error) {
	tracer := &otelTracer{Config: config}

	mp := metric.NewNoopMeterProvider()
	global.SetMeterProvider(mp)

	exp, err := NewLogFileExporter(tracer.Port)
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
		sdktrace.WithSpanProcessor(Annotator{
			globalTags: tracer.GlobalTags,
			sampleRate: 1,
		}),
	}

	tp := sdktrace.NewTracerProvider(tpOptions...)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	tracer.serviceName = serviceName
	tracer.tracerProvider = tp

	tracer.tracerProvider.Tracer(serviceName)

	return tracer, nil
}

func NewLogFileExporter(port int) (sdktrace.SpanExporter, error) {
	conn, err := net.Dial(logfile.TraceSocketType, fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}

	return &LogFileSpanExporter{conn: conn}, nil
}

type LogFileSpanExporter struct {
	conn net.Conn
}

func (se *LogFileSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	fmt.Println("export spans")
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		stubs := tracetest.SpanStubsFromReadOnlySpans(spans)
		fmt.Printf("stubs: %#v\n", stubs)
		return json.NewEncoder(se.conn).Encode(stubs)
	}
}

func (se *LogFileSpanExporter) Shutdown(ctx context.Context) error {
	return se.conn.Close()
}
