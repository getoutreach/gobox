# ometrics

```go
import "github/getoutreach/gobox/pkg/ometrics"
```

Package ometrics implements a small wrapper around working with the otel metrics package. It does not provide any wrappers around the core types provided by otel, instead provides a way to instantiate them instead.

## Index

- [Usage](#usage)
- [func InitializeMeterProvider(ctx context.Context, t ExporterType, opts ...Option) error](<#func-initializemeterprovider>)
- [type CollectorConfig](<#type-collectorconfig>)
- [type Config](<#type-config>)
- [type ExporterType](<#type-exportertype>)
- [type Option](<#type-option>)
  - [func WithConfig(c Config) Option](<#func-withconfig>)

## Usage

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
