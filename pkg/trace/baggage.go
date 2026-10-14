package trace

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// maxBaggageValueLen caps copied baggage values to bound attribute size.
const maxBaggageValueLen = 256

// BaggageAllowList maps allowed baggage keys to the span attribute name they
// are written to. Never add PII or PHI keys: baggage crosses trust boundaries.
// nolint:gochecknoglobals // Why: constant-like lookup table
var BaggageAllowList = map[string]string{
	"graphql.operation.name": "graphql.operation.name",
	"ScenarioName":           "scenario.name",
}

// BaggageSpanProcessor copies allowlisted baggage members onto every span at start.
type BaggageSpanProcessor struct {
	allow map[string]string
}

// NewBaggageSpanProcessor returns a processor for the given allowlist
// (baggage key -> attribute name); nil uses BaggageAllowList.
func NewBaggageSpanProcessor(allow map[string]string) BaggageSpanProcessor {
	if allow == nil {
		allow = BaggageAllowList
	}
	return BaggageSpanProcessor{allow: allow}
}

func (p BaggageSpanProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
	members := baggage.FromContext(ctx).Members()
	for _, m := range members {
		attr, ok := p.allow[m.Key()]
		if !ok || m.Value() == "" {
			continue
		}
		v := m.Value()
		if len(v) > maxBaggageValueLen {
			v = v[:maxBaggageValueLen]
		}
		s.SetAttributes(attribute.String(attr, v))
	}
}

func (BaggageSpanProcessor) OnEnd(sdktrace.ReadOnlySpan)      {}
func (BaggageSpanProcessor) Shutdown(context.Context) error   { return nil }
func (BaggageSpanProcessor) ForceFlush(context.Context) error { return nil }
