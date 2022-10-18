package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	osexec "os/exec"
	"os/user"
	"runtime"
	"strings"

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

// NewLogFileTracer initializes a tracer that sends traces to a log file.
func NewLogFileTracer(ctx context.Context, serviceName string, config *Config) (tracer, error) {
	tracer := &otelTracer{Config: *config}

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

// NewLogFileExporter Creates a new exporter that sends all spans to the passed in port
// on localhost.
func NewLogFileExporter(port int) (sdktrace.SpanExporter, error) {
	conn, err := net.Dial(logfile.TraceSocketType, fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}

	return &LogFileSpanExporter{conn: conn}, nil
}

// LogFileSpanExporter an exporter that sends all traces across the configured connection.
type LogFileSpanExporter struct {
	conn net.Conn
}

// ExportSpans exports all the provided spans.
func (se *LogFileSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	fmt.Println("export spans")
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		stubs := tracetest.SpanStubsFromReadOnlySpans(spans)
		return json.NewEncoder(se.conn).Encode(stubs)
	}
}

// Shutdown cleans up when the exporter close by ensuring that the connection gets closed.
func (se *LogFileSpanExporter) Shutdown(ctx context.Context) error {
	return se.conn.Close()
}

// CommonProps sets up common properties for traces in cli apps
func CommonProps() log.Marshaler {
	commonProps := log.F{
		"os.name": runtime.GOOS,
		"os.arch": runtime.GOARCH,
	}
	if b, err := osexec.Command("git", "config", "user.email").Output(); err == nil {
		email := strings.TrimSuffix(string(b), "\n")

		// TODO: Turn the check into an config option
		// In case of @outreach.io email, we want to add PII for easier debugging with devs
		if strings.HasSuffix(email, "@outreach.io") {
			commonProps["dev.email"] = email

			if u, err := user.Current(); err == nil {
				commonProps["os.user"] = u.Username
			}

			if hostname, err := os.Hostname(); err == nil {
				commonProps["os.hostname"] = hostname
			}
			path, err := os.Getwd()
			if err == nil {
				commonProps["os.workDir"] = path
			}
		}
	}

	return commonProps
}

// SetupLogFileTracer initializes tracing for clis
func SetupLogFileTracer(ctx context.Context, name string) context.Context {
	if err := InitTracer(ctx, name); err != nil {
		return ctx
	}
	return StartSpan(ctx, name)
}
