package trace

import (
	"context"
	"os"
	"sync"

	"github.com/getoutreach/gobox/pkg/log"
)

type tracer struct {
	Config
	sync.Once
}

// Deprecated: Use initTracer() instead.
func (t *tracer) startTracing(serviceName string) error {
	return t.initTracer(context.TODO(), serviceName)
}

func (t *tracer) initTracer(ctx context.Context, serviceName string) error {
	if err := t.Config.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := t.startHoneycomb(ctx, serviceName); err != nil {
		return err
	}
	return nil
}

// Deprecated: Use closeTracer() instead.
func (t *tracer) endTracing() {
	t.stopHoneycomb(context.TODO())
}

func (t *tracer) closeTracer(ctx context.Context) {
	t.stopHoneycomb(ctx)
}

func (t *tracer) startTrace(ctx context.Context, name string) context.Context {
	return t.startHoneycombTrace(ctx, name, nil)
}

func (t *tracer) startSpan(ctx context.Context, name string) context.Context {
	return t.startHoneycombSpan(ctx, name)
}

func (t *tracer) end(ctx context.Context) {
	t.endHoneycombSpan(ctx)
}

func (t *tracer) addInfo(ctx context.Context, args ...log.Marshaler) {
	t.addHoneycombFields(ctx, args...)
}

func (t *tracer) id(ctx context.Context) string {
	return t.honeycombTraceID(ctx)
}
