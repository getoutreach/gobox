// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the honeycomb trace implementation.

package tracer

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/honeycombio/beeline-go"
	"github.com/honeycombio/beeline-go/propagation"
	"github.com/honeycombio/beeline-go/sample"
	"github.com/honeycombio/beeline-go/trace"
	"github.com/pkg/errors"
)

// Note: this package does not depend on pkg/log or pkg/trace to avoid
// circular dependencies.

const (
	// special honeycomb fields
	fieldTraceID     = "trace.trace_id"
	fieldForceTrace  = "force_trace"
	fieldSampleTrace = "sample_trace"

	// special HTTP/GRPC headers
	forceTracingHeader = "X-Force-Trace"
)

// WithHoneycomb adds the honeycomb tracer..
func WithHoneycomb() Option {
	return WithProvider(&honeycomb{})
}

// honeycomb implements the tracing interface.
type honeycomb struct {
	Tracer     *Tracer
	globalTags map[string]interface{}
	presend    func(map[string]interface{})
}

// Init initializes the beeline config.
func (h *honeycomb) Init(ctx context.Context, t *Tracer, serviceName string) error {
	var config struct {
		Honeycomb struct {
			Enabled       bool       `yaml:"Enabled"`
			APIHost       string     `yaml:"APIHost"`
			Dataset       string     `yaml:"Dataset"`
			SamplePercent float64    `yaml:"SamplePercent"`
			Debug         bool       `yaml:"Debug"`
			Stdout        bool       `yaml:"Stdout"`
			APIKey        cfg.Secret `yaml:"APIKey"`
		} `yaml:"Honeycomb"`
		GlobalTags struct {
			DevEmail string `yaml:"DevEmail,omitempty"`
		} `yaml:"GlobalTags,omitempty"`
	}

	if err := cfg.Load("trace.yaml", &config); err != nil && !os.IsNotExist(err) {
		return err
	}

	if !config.Honeycomb.Enabled {
		return nil
	}

	key, err := config.Honeycomb.APIKey.Data(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to fetch api key")
	}

	beeline.Init(beeline.Config{
		APIHost:     config.Honeycomb.APIHost,
		WriteKey:    strings.TrimSpace(string(key)),
		Dataset:     config.Honeycomb.Dataset,
		ServiceName: serviceName,
		// honeycomb accepts sample rates as number of requests seen
		// per request sampled
		SamplerHook: h.samplerHook(uint(100 / config.Honeycomb.SamplePercent)),
		PresendHook: h.presendHook,
		Debug:       config.Honeycomb.Debug,
		STDOUT:      config.Honeycomb.Stdout,
	})

	if config.GlobalTags.DevEmail != "" {
		h.globalTags = map[string]interface{}{
			"dev.mail": config.GlobalTags.DevEmail,
		}
	}

	h.Tracer = t
	return nil
}

// Close cleans up the beeline config, flushing any events.
func (h *honeycomb) Close(ctx context.Context) {
	if h.Tracer != nil {
		beeline.Flush(ctx)
		beeline.Close()
	}
}

// SetPresendHook provides a way for tests to hook into the raw events.
func (h *honeycomb) SetPresendHook(hook func(map[string]interface{})) {
	h.presend = hook
}

func (h *honeycomb) presendHook(fields map[string]interface{}) {
	for k, v := range h.globalTags {
		fields[k] = v
	}
	logf.F(fields).Set("", app.Info())
	if h.presend != nil {
		h.presend(fields)
	}
}

// ForceTrace enables unsampled tracing.
func (h *honeycomb) ForceTrace(ctx context.Context) context.Context {
	if t := trace.GetTraceFromContext(ctx); t != nil {
		t.AddField(fieldForceTrace, "true")
	}
	return ctx
}

// SetCurrentSampleRate sets the current sample rate.
//
// The rate is specified as an inverse: i.e. if N is specified, one
// sample in N is used.
//
// If N = 1, this forces full tracing.
func (h *honeycomb) SetCurrentSampleRate(ctx context.Context, rate uint) context.Context {
	if t := trace.GetTraceFromContext(ctx); t != nil {
		t.AddField(fieldSampleTrace, rate)
	}
	return ctx
}

func (h *honeycomb) samplerHook(rate uint) func(map[string]interface{}) (bool, int) {
	sampler, err := sample.NewDeterministicSampler(rate)
	if err != nil {
		panic(err)
	}

	return func(fields map[string]interface{}) (bool, int) {
		if _, ok := fields[fieldForceTrace]; ok {
			return true, 1
		}

		if rawRate, ok := fields[fieldSampleTrace]; ok {
			if rate, ok := rawRate.(uint); ok {
				return true, int(rate)
			}
		}

		if traceID, ok := fields[fieldTraceID].(string); ok {
			return sampler.Sample(traceID), sampler.GetSampleRate()
		}
		return false, 0
	}
}

// StartTrace starts a new trace with the provided headers.  If if
// finds the relevant trace headers, it uses that to create a child
// trace otherwise it creates a new trace.
//
// The returned context contains a root span.
func (h *honeycomb) StartTrace(ctx context.Context, name string, headers map[string][]string) context.Context {
	if h.Tracer != nil {
		var t *trace.Trace
		header := http.Header(headers).Get(propagation.TracePropagationHTTPHeader)
		// nolint:errcheck // Why: we can ignore bad/malformed headers
		prop, _ := propagation.UnmarshalHoneycombTraceContext(header)
		ctx, t = trace.NewTrace(ctx, prop)
		t.GetRootSpan().AddField("name", name)
		if _, ok := headers[forceTracingHeader]; ok {
			// TODO:  This behavior is a bit of a risk for public endpoints
			// as unauthenticated clients can use this header and run a form
			// of the DoS attack. Ideally, public endpoints can be identified
			// and the force tracing header is only applied *after*
			// authentication.
			t.AddField(fieldForceTrace, "true")
		}
	}
	return ctx
}

// EndTrace ends the current trace.
func (h *honeycomb) EndTrace(ctx context.Context) {
	// TODO: check if the current span is the root span and report
	// incorrect span completion.
	h.EndSpan(ctx, SpanSync)
}

func (h *honeycomb) AddTraceInfo(ctx context.Context, info logf.Marshaler) {
	if t := trace.GetTraceFromContext(ctx); t != nil {
		// TODO:  should this be added direclty on the trace itself?
		logf.Marshal("", info, t.GetRootSpan().AddField)
	}
}

// StartSpan starts a new span and sets it up in a derived context
// which is returned.
func (h *honeycomb) StartSpan(ctx context.Context, name string, args logf.Marshaler, spanType SpanType) context.Context {
	span := trace.GetSpanFromContext(ctx)
	if span == nil {
		return ctx
	}

	fmt.Fprintln(os.Stderr, "Creating new span from", span.GetSpanID(), name)

	// TODO: For incoming calls, we can reuse the root span here.
	// To do that, we need to update all callsites to specify the
	// call type right away (and modify this code to handle that).
	if spanType == SpanAsync {
		ctx, span = span.CreateAsyncChild(ctx)
	} else {
		ctx, span = span.CreateChild(ctx)
	}

	fmt.Fprintln(os.Stderr, "Created new span", span.GetSpanID())

	span.AddField("name", name)
	logf.Marshal("", args, span.AddField)
	return ctx
}

func (h *honeycomb) EndSpan(ctx context.Context, spanType SpanType) {
	var info logf.Marshaler

	if h.Tracer != nil && spanType.IsCall() {
		info = h.Tracer.Info(ctx).Call
	}

	if span := trace.GetSpanFromContext(ctx); span != nil {
		logf.Marshal("", info, span.AddField)
		fmt.Fprintln(os.Stderr, "Sending SpanID", span.GetSpanID())
		span.Send()

		// TODO: remove the following behavior and move it to EndTrace.
		if span.GetParent() == nil {
			fmt.Fprintln(os.Stderr, "Sending trace")
			trace.GetTraceFromContext(ctx).Send()
		}
	}
}

func (h *honeycomb) AddSpanInfo(ctx context.Context, spanType SpanType, info logf.Marshaler) {
	if span := trace.GetSpanFromContext(ctx); span != nil {
		logf.Marshal("", info, span.AddField)
	}
}

func (h *honeycomb) SetSpanStatus(ctx context.Context, spanType SpanType, err error) {
	h.AddSpanInfo(ctx, spanType, events.Err(err))
}

func (h *honeycomb) CurrentInfo(ctx context.Context, info *Info) {
	if t := trace.GetTraceFromContext(ctx); t != nil {
		// TODO: Document why we need this prefix here.
		info.TraceID = "hctrace_" + t.GetTraceID()
	}
	if span := trace.GetSpanFromContext(ctx); span != nil {
		info.SpanID = span.GetSpanID()
		if info.ParentID = span.GetParentID(); info.ParentID == "" {
			info.ParentID = info.SpanID
		}
	}
}

func (h *honeycomb) CurrentHeaders(ctx context.Context, headers map[string][]string) {
	if span := trace.GetSpanFromContext(ctx); span != nil {
		value := []string{span.SerializeHeaders()}
		headers[propagation.TracePropagationHTTPHeader] = value

		// We do not actively propagate the force tracing header and
		// the sample rate headers.
		// TODO: propagate those as honeycomb documentation claims they do
		// not implement head-based sampling.
		// See: https://docs.honeycomb.io/getting-data-in/tracing/sampling/
	}
}
