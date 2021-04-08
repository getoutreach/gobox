package trace

import (
	"context"
	"strings"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	beeline "github.com/honeycombio/beeline-go"
	"github.com/honeycombio/beeline-go/trace"
)

// nolint:gochecknoglobals
var testPresendHook func(map[string]interface{})

// SetTestPresendHook sets the honeycomb presend hook for testing
func SetTestPresendHook(hook func(map[string]interface{})) {
	testPresendHook = hook
}

func (t *tracer) startHoneycomb(serviceName string) error {
	if !t.Honeycomb.Enabled {
		return nil
	}

	ctx := context.Background()
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
		SampleRate:  uint(100 / t.Honeycomb.SamplePercent),
		PresendHook: testPresendHook,
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

func (t *tracer) startHoneycombTrace(ctx context.Context, name string) context.Context {
	if !t.Honeycomb.Enabled {
		return ctx
	}

	ctx, tr := trace.NewTrace(ctx, "")
	marshalLog(tr.AddField, "", app.Info())
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
			marshalLog(span.AddField, "", f)
		}
	}
}

func marshalLog(setf func(key string, v interface{}), key string, l log.Marshaler) {
	if l == nil {
		return
	}
	l.MarshalLog(func(innerKey string, innerValue interface{}) {
		if innerValue == nil {
			return
		}

		if key != "" {
			innerKey = key + "." + innerKey
		}
		if mm, ok := innerValue.(log.Marshaler); ok {
			// recurse with the keys combined
			marshalLog(setf, innerKey, mm)
		} else {
			setf(innerKey, innerValue)
		}
	})
}
