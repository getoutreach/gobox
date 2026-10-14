package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestBaggageSpanProcessor(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(NewBaggageSpanProcessor(nil)),
		sdktrace.WithSpanProcessor(rec),
	)
	mk := func(k, v string) baggage.Member {
		m, err := baggage.NewMemberRaw(k, v)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	b, _ := baggage.New(mk("client_operation_name", "GetFoo"), mk("ScenarioName", "Nav"), mk("user.email", "a@b.c"))
	ctx := baggage.ContextWithBaggage(context.Background(), b)
	_, span := tp.Tracer("t").Start(ctx, "x")
	span.End()

	got := map[string]string{}
	for _, kv := range rec.Ended()[0].Attributes() {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	if got["graphql.operation.name"] != "GetFoo" || got["scenario.name"] != "Nav" {
		t.Fatalf("missing allowlisted attrs: %v", got)
	}
	if _, ok := got["user.email"]; ok {
		t.Fatal("non-allowlisted key copied")
	}
}
