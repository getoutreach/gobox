# ometrics

```go
import "github/getoutreach/gobox/pkg/ometrics"
```

Package ometrics implements a small wrapper around working with the OpenTelemetry (otel) metrics package. It does not provide any wrappers around the core types provided by otel, but instead provides a way to instantiate them.

## Index

- [Usage](#usage)
- [Migrating from gobox/pkg/metrics](<#migrating-from-goboxpkgmetrics>)
- [func InitializeMeterProvider(ctx context.Context, t ExporterType, opts ...Option) error](<#func-initializemeterprovider>)
- [type CollectorConfig](<#type-collectorconfig>)
- [type Config](<#type-config>)
- [type ExporterType](<#type-exportertype>)
- [type Option](<#type-option>)
  - [func WithConfig(c Config) Option](<#func-withconfig>)

## Usage

### Initialize metrics exporter

In order to ensure the correct provider is used to export the open-telemetry metrics, it is required to call the `gobox/pkg/ometrics` func `InitializeMeterProvider(...)` with the expected provider type as well as any other relevant options. Currently, this package only has support for `prometheus` and `otlp` (open-telemetry collector) exporter providers. Furthermore, the `prometheus` provider type is the only type which will be referenced throughout these docs, as it will provide the closest examples to current metric usage.

The below code demonstrates how to initialize the `prometheus` provider using this package, which only needs to be done once, ideally in some service setup file *(`main.go`, `internal/[serviceName]/server.go`, etc.)* Likely this will be added to a stencil template in the future, but will need to be added manually for now.

```go
// main.go

import (
    "context"

    "github.com/getoutreach/gobox/pkg/ometrics"
)

func main() {
    ctx := context.Background()

    ...

    if err := ometrics.InitializeMeterProvider(ctx, ometrics.ExporterTypePrometheus); err != nil {
        // Handle err
    }
}
```

> [!IMPORTANT]
> While this function will initialize the default, global provider for the open-telemetry package, it will not automatically create or expose an HTTP handler for consuming these metrics. This will still need to be done by the caller. However, as long as your service is using the `stencil-golang` module, this should be provided for you through the `github.com/getoutreach/httpx` package. If your service is not using the template, then you will likely still need to create and configure an HTTP endpoint for your metrics to be consumed through.

### Setup package level meter (_recommended usage_)

In order to setup instruments on which metrics may be recorded, a meter is first required. A meter ties the instrumented recordings to a scope and finally to the configured provider which will handle the exporting of those recorded metrics. An important note here is that these meters are intended to be **scoped**. According to the open-telemetry documentation, a meter should be scoped to a package. Because of this, it is expected by convention to use the package name of the calling code for this meter. See docs here: https://pkg.go.dev/go.opentelemetry.io/otel/metric#MeterProvider. *(The scope name from the meter simply gets added as an attribute on prometheus observations).*

> [!IMPORTANT]
> Using the open-telemetry package's global `Meter` func will use whichever provider is currently configured, and the resulting meter will be tied to that provider. Because of this, it is important that you have called the `ometrics.InitializeMeterProvider` before creating any meters.

```go
import (
    "context"

    "github.com/getoutreach/gobox/pkg/ometrics"
    "go.opentelemetry.io/otel/metric"
)

// The otel meter's ideally use the package name of the caller to
// scope any instruments created by that meter to this package, as
// opposed to other packages/code.
const packageName = "github.com/getoutreach/serviceName/pkg/pkgName"

// For ease of use, we will have a package-level our meter which
// can be readily accessed.
var meter metric.Meter

// This func ensures our package-level meter is created with any
// necessary configurations. This is especially important in order
// ensure that our meter is not created before we have called the
// `ometrics.InitializeMeterProvider` func.
func createPackageMeter() {
    meter = otel.Meter(packageName)
}
```

### Creating an instrument

```go
import (
    "context"

    "github.com/getoutreach/gobox/pkg/ometrics"
    "go.opentelemetry.io/otel/metric"
)

// The otel meter's ideally use the package name of the caller to
// scope any instruments created by that meter to this package, as
// opposed to other packages/code.
const packageName = "github.com/getoutreach/serviceName/pkg/pkgName"

// For ease of use, we will have a package-level our meter which
// can be readily accessed.
var meter metric.Meter

// We will create a package level histogram instrument for call
// latency.
var exampleLatencyInstr metric.Float64Histogram

func initMetrics() {
    // Ensure the ometrics provider is initialized.
    if err := ometrics.InitializeMeterProvider(ctx, ometrics.ExporterTypePrometheus); err != nil {
        // Handle err
    }

    // Create a package scoped meter.
    meter = otel.Meter(packageName)

    // Create a float64 histogram instrument from out meter with
    // a description and appropriate unit ('s' for seconds).
    exampleLatencyInstr = meter.Float64Histogram(
        "example_call_seconds",
        metric.WithDescription("The latency of example func calls, in seconds"),
        metric.WithUnit("s"),
    )
}
```

### Recording a value using an instrument

This example usage uses the previous section as the implied setup for this usage.

```go
import (
    "context"
    "time"
)

// Setup metrics
...

func exampleFunc(ctx context.Context, req interface{}) error {
    start := time.Now()

    // Do some work
    ...

    // Record the diff in time between start and now with our example
    // func latency histogram instrument.
    took := time.Since(start)
    exampleLatencyInstr.Record(ctx, took.Seconds())
}

```

### Further Reading

These docs provide high level usage examples, and specific information about the usage of otel with this package. However, most metrics instrumentation will ultimately be done directly using the Golang `otel` package. To get a full understanding of the available instruments and their usage, check out the docs: https://pkg.go.dev/go.opentelemetry.io/otel/metric.

## Migrating from `gobox/pkg/metrics`

Today, the current `metrics` package only exposes a small set of functionality, primarily creating a few basic http/grpc histograms. However, most code outside of the `github.com/getoutreach/httpx` package do not seem to use the `metrics` package directly (though all services using Stencil should likely be using the `httpx` package's default metrics, if applicable). Most seem to use the Prometheus libraries directly. Fortunately, the open-telemetry metrics package should allow for a progressive switch over to the new package, as both direct usage of Prometheus as well as usage through open-telemetry can be done concurrently. New instruments can be created with open-telemetry package while old packages using the Prometheus libraries directly can begin to gradually be moved over.

Below is an example of using the new open-telemetry package using a common pattern found within our own codebases.

*new*

```go
// github.com/getoutreach/serviceName/internal/metrics/metrics.go
package metrics

import (
    "context"
    "time.Time"

    "github.com/getoutreach/gobox/pkg/ometrics"
    "go.opentelemetry.io/otel/metric"
)

const packageName = "github.com/getoutreach/serviceName/internal/metrics"

var (
    meter metric.Meter
    exampleLatencyInstr metric.Float64Histogram
)

func initMetrics() {
    if err := ometrics.InitializeMeterProvider(ctx, ometrics.ExporterTypePrometheus); err != nil {
        // Handle err
    }

    meter = otel.Meter(packageName)

    exampleLatencyInstr = meter.Float64Histogram(
        "example_call_seconds",
        metric.WithDescription("The latency of example func calls, in seconds"),
        metric.WithUnit("s"),
    )
}

func ReportExampleLatency(ctx context.Context, d time.Duration) {
    exampleLatencyInstr.Record(ctx, d.Seconds())
}
```

*old*

```go
// github.com/getoutreach/serviceName/internal/metrics/metrics.go
package metrics

import (
    "strconv"
    "time"

    "github.com/prometheus/client_golang/prometheus"
)

func init() {
    prometheus.MustRegister(exampleLatencyInstr)
}

var exampleLatencyInstr = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "example_call_seconds",
        Help: "The latency of example func calls, in seconds",
    }, []string{})

func ReportExampleLatency(subject, operation string, err error, d time.Duration) {
    exampleLatencyInstr.Observe(d.Seconds())
}
```

## func [InitializeMeterProvider](<https://github.com/getoutreach/gobox/blob/main/pkg/ometrics/ometrics.go#L45>)

```go
func InitializeMeterProvider(ctx context.Context, t ExporterType, opts ...Option) error
```

InitializeMeterProvider initializes the global meter provider to be backed by the provided exporter.

## type [CollectorConfig](<https://github.com/getoutreach/gobox/blob/main/pkg/ometrics/config.go#L19-L23>)

CollectorConfig contains configuration for creating a ExporterTypeCollector exporter through InitializeMeterProvider.

```go
type CollectorConfig struct {
    // Interval is the time at which metrics should be read and
    // subsequently pushed to the collector.
    Interval time.Duration
}
```

## type [Config](<https://github.com/getoutreach/gobox/blob/main/pkg/ometrics/config.go#L11-L15>)

Config is the configuration for a meter provider created by this package. This is meant to be used by InitializeMeterProvider.

```go
type Config struct {
    // Collector contains configuration for the collector exporter. This
    // is only valid when using the ExporterTypeCollector.
    Collector CollectorConfig
}
```

## type [ExporterType](<https://github.com/getoutreach/gobox/blob/main/pkg/ometrics/ometrics.go#L27>)

ExporterType denotes the type of exporter to use for metrics.

```go
type ExporterType int
```

Contains the different types of exporters that can be used.

```go
const (
    // ExporterTypePrometheus exports metrics in the prometheus format. It
    // is the caller's responsibility to expose the metrics via an HTTP
    // endpoint (usually through promhttp). Example implementation:
    // https://github.com/open-telemetry/opentelemetry-go/blob/main/example/prometheus/main.go#L99
    ExporterTypePrometheus ExporterType = iota

    // ExporterTypeCollector exports metrics to the otel collector. This
    // is akin to the "push" model of metrics, for those familiar with
    // the prometheus model.
    ExporterTypeCollector
)
```

## type [Option](<https://github.com/getoutreach/gobox/blob/main/pkg/ometrics/config.go#L26>)

Option is a function that sets a configuration value.

```go
type Option func(c *Config)
```

### func [WithConfig](<https://github.com/getoutreach/gobox/blob/main/pkg/ometrics/config.go#L30>)

```go
func WithConfig(c Config) Option
```

WithConfig sets the configuration for a meter provider replacing all default values.
