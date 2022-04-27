package trace

import (
	"context"
	"strings"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/honeycombio/beeline-go"
	"github.com/honeycombio/beeline-go/propagation"
	"github.com/honeycombio/beeline-go/trace"
)

// nolint:gochecknoglobals
var presendHook func(map[string]interface{})

// SetPresendHook sets the honeycomb presend hook for testing
func SetPresendHook(hook func(map[string]interface{})) {
	presendHook = hook
}

func (t *tracer) startHoneycomb(ctx context.Context, serviceName string) error {
	if !t.Honeycomb.Enabled {
		return nil
	}

	key, err := t.Honeycomb.APIKey.Data(ctx)
	if err != nil {
		log.Error(ctx, "Unable to fetch honeycomb API key", events.NewErrorInfo(err))
		return err
	}

	beeline.Init(beeline.Config{
		APIHost:     t.Honeycomb.APIHost,
		WriteKey:    strings.TrimSpace(string(key)),
		Dataset:     t.Honeycomb.Dataset,
		ServiceName: serviceName,
		// honeycomb accepts sample rates as number of requests seen
		// per request sampled
		SamplerHook: forceSampler(uint(100 / t.Honeycomb.SamplePercent)),
		PresendHook: t.presendHook,
		Debug:       t.Honeycomb.Debug,
		STDOUT:      t.Honeycomb.Stdout,
	})
	return nil
}

func (t *tracer) stopHoneycomb(ctx context.Context) {
	if !t.Honeycomb.Enabled {
		return
	}

	beeline.Flush(ctx)
	beeline.Close()
}

func (t *tracer) presendHook(fields map[string]interface{}) {
	setf := func(key string, value interface{}) {
		fields[key] = value
	}

	// Set service-level tags on every single span we send.
	logf.Marshal("", app.Info(), setf)
	logf.Marshal("", &t.GlobalTags, setf)

	if presendHook != nil {
		presendHook(fields)
	}
}

func (t *tracer) startHoneycombTrace(ctx context.Context, name string, prop *propagation.PropagationContext) context.Context {
	if !t.Honeycomb.Enabled {
		return ctx
	}

	ctx, tr := trace.NewTrace(ctx, prop)
	tr.GetRootSpan().AddField("name", name)
	return ctx
}

func (t *tracer) honeycombTraceID(ctx context.Context) string {
	if t := trace.GetTraceFromContext(ctx); t != nil {
		return "hctrace_" + t.GetTraceID()
	}
	return ""
}

func (t *tracer) startHoneycombSpan(ctx context.Context, name string) context.Context {
	if span := trace.GetSpanFromContext(ctx); span != nil {
		ctx, span = span.CreateChild(ctx)
		span.AddField("name", name)
	}
	return ctx
}

func (t *tracer) startHoneycombSpanAsync(ctx context.Context, name string) context.Context {
	if span := trace.GetSpanFromContext(ctx); span != nil {
		ctx, span = span.CreateAsyncChild(ctx)
		span.AddField("name", name)
	}
	return ctx
}

func (t *tracer) endHoneycombSpan(ctx context.Context) {
	if span := trace.GetSpanFromContext(ctx); span != nil {
		span.Send()
		if span.GetParent() == nil {
			trace.GetTraceFromContext(ctx).Send()
		}
	}
}

func (t *tracer) addHoneycombFields(ctx context.Context, args ...log.Marshaler) {
	if span := trace.GetSpanFromContext(ctx); span != nil {
		for _, f := range args {
			logf.Marshal("", f, span.AddField)
		}
	}
}

func (t *tracer) honeycombParentID(ctx context.Context) string {
	if t := trace.GetSpanFromContext(ctx); t != nil {
		if parentID := t.GetParentID(); parentID != "" {
			return parentID
		}
		return t.GetSpanID()
	}
	return ""
}

func (t *tracer) honeycombSpanID(ctx context.Context) string {
	if t := trace.GetSpanFromContext(ctx); t != nil {
		return t.GetSpanID()
	}
	return ""
}
