// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file implements the tracing storage entry
// for the otel format.

package logfile

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Span is a type similar to otel's SpanStub, but with the correct types needed
// for handle marshalling and unmarshalling.
type Span struct {
	// Name is the name of a the specific span
	Name string
	// SpanContext is the unique SpanContext that identifies the span
	SpanContext trace.SpanContext
	// Parten is the unique SpanContext that identifies the parent of the span.
	// If the span has no parent, this span context will be invalid.
	Parent trace.SpanContext
	// SpanKind is the role the span plays in a Trace
	SpanKind trace.SpanKind
	// StartTime is the time the span started recording
	StartTime time.Time
	// EndTime returns the time the span stopped recording
	EndTime time.Time
	// Attributes are the defining attributes of a span
	Attributes []attribute.KeyValue
	// Events are all the events that occurred within the span
	Events []tracesdk.Event
	// Links are all the links the span has to other spans
	Links []tracesdk.Link
	// Status is that span status
	Status tracesdk.Status
	// DroppedAttributes is the number of attributes dropped by the span due to a limit being reached
	DroppedAttributes int
	// DroppedEvents is the number of attributes dropped by the span due to a limit being reached
	DroppedEvents int
	// DroppedLinks is the number of links dropped by the span due to a limit being reached
	DroppedLinks int
	// ChildSpanCount is the count of spans that consider the span a direct parent
	ChildSpanCount int
	// Resource is the information about the entity that produced the span
	// We have to change this type from the otel type in order to make this struct marshallable
	Resource []attribute.KeyValue
	// InstrumentationLibrary is information about the library that produced the span
	InstrumentationLibrary instrumentation.Library
}

// spanData is data that we need to unmarshal in custom ways
type spanData struct {
	Name        string
	SpanContext spanContext
	Parent      spanContext
	SpanKind    trace.SpanKind
	StartTime   time.Time
	EndTime     time.Time
	Attributes  []keyValue
	// Events will currently get dropped
	Events []tracesdk.Event
	// Links will currently get dropped
	Links             []tracesdk.Link
	Status            tracesdk.Status
	DroppedAttributes int
	DroppedEvents     int
	DroppedLinks      int
	ChildSpanCount    int
	// We have to change this type from the otel type in order to make this struct marshallable
	Resource               []keyValue
	InstrumentationLibrary instrumentation.Library
}

// spanContext is a custom type used to unmarshal otel SpanContext correctly
type spanContext struct {
	TraceID    string
	SpanID     string
	TraceFlags string
	// TraceState will currenctly get dropped
	TraceState string
	Remote     bool
}

// keyValue is a custom type used to unmarshal otel KeyValue correctly
type keyValue struct {
	Key   string
	Value value
}

// value is a custom type used to unmarshal otel Value correctly
type value struct {
	Type  string
	Value interface{}
}

// UnmarshalJSON implements json.Unmarshaler for Span which allows
// correctly retrieving attribute.KeyValue values
func (s *Span) UnmarshalJSON(data []byte) error {
	var sd spanData
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&sd); err != nil {
		return errors.Wrap(err, "unable to decode to spanData")
	}

	// Set values that translate directly
	s.Name = sd.Name
	s.SpanKind = sd.SpanKind
	s.StartTime = sd.StartTime
	s.EndTime = sd.EndTime
	s.Status = sd.Status
	s.DroppedAttributes = sd.DroppedAttributes
	s.DroppedEvents = sd.DroppedEvents
	s.DroppedLinks = sd.DroppedLinks
	s.ChildSpanCount = sd.ChildSpanCount
	s.InstrumentationLibrary = sd.InstrumentationLibrary

	// Create the correct SpanContext attributes
	spanContext, err := sd.SpanContext.asTraceSpanContext()
	if err != nil {
		return errors.Wrap(err, "unable to decode spanContext")
	}
	s.SpanContext = spanContext

	parent, err := sd.Parent.asTraceSpanContext()
	if err != nil {
		return errors.Wrap(err, "unable to decode parent")
	}
	s.Parent = parent

	var attributes []attribute.KeyValue
	for _, a := range sd.Attributes {
		kv, err := a.asAttributeKeyValue()
		if err != nil {
			return errors.Wrapf(err, "unable to decode attribute (%s)", a.Key)
		}
		attributes = append(attributes, kv)
	}
	s.Attributes = attributes

	var resources []attribute.KeyValue
	for _, r := range sd.Resource {
		kv, err := r.asAttributeKeyValue()
		if err != nil {
			return errors.Wrapf(err, "unable to decode resource (%s)", r.Key)
		}
		resources = append(resources, kv)
	}
	s.Resource = resources

	return nil
}

// asTraceSpanContext converst the internal spanContext representation to an otel one
func (sc *spanContext) asTraceSpanContext() (trace.SpanContext, error) {
	traceID, err := traceIDFromHex(sc.TraceID)
	if err != nil {
		return trace.SpanContext{}, errors.Wrap(err, "unable to parse trace id")
	}

	spanID, err := spanIDFromHex(sc.SpanID)
	if err != nil {
		return trace.SpanContext{}, errors.Wrap(err, "unable to parse span id")
	}

	traceFlags := trace.TraceFlags(0x00)
	if sc.TraceFlags == "01" {
		traceFlags = trace.TraceFlags(0x01)
	}

	config := trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: traceFlags,
		Remote:     sc.Remote,
	}

	return trace.NewSpanContext(config), nil
}

// asAttributeKeyValue converts the internal keyValue representation to an otel one
func (kv *keyValue) asAttributeKeyValue() (attribute.KeyValue, error) {
	// Value types get encoded as string
	switch kv.Value.Type {
	case attribute.INVALID.String():
		return attribute.KeyValue{}, errors.New("invalid value type")
	case attribute.BOOL.String():
		return attribute.Bool(kv.Key, kv.Value.Value.(bool)), nil
	case attribute.INT64.String():
		// Value could be int64 or float64, so handle
		// both cases. float64 comes from json.Unmarshal.
		var v int64
		switch i := kv.Value.Value.(type) {
		case int64:
			v = i
		case float64:
			v = int64(i)
		}
		return attribute.Int64(kv.Key, v), nil
	case attribute.FLOAT64.String():
		return attribute.Float64(kv.Key, kv.Value.Value.(float64)), nil
	case attribute.STRING.String():
		return attribute.String(kv.Key, kv.Value.Value.(string)), nil
	case attribute.BOOLSLICE.String():
		return attribute.BoolSlice(kv.Key, kv.Value.Value.([]bool)), nil
	case attribute.INT64SLICE.String():
		// Handle both float64 and int64. float64 comes from json.Unmarshal.
		var v []int64
		switch sli := kv.Value.Value.(type) {
		case []int64:
			v = sli
		case []float64:
			for i := range sli {
				v = append(v, int64(sli[i]))
			}
		}

		return attribute.Int64Slice(kv.Key, v), nil
	case attribute.FLOAT64SLICE.String():
		return attribute.Float64Slice(kv.Key, kv.Value.Value.([]float64)), nil
	case attribute.STRINGSLICE.String():
		var strSli []string

		// Sometimes we can get a []interface{} instead of a []string, so always cast
		// to []string if that happens.
		switch sli := kv.Value.Value.(type) {
		case []string:
			strSli = sli
		case []interface{}:
			for i := range sli {
				var v string

				// Best case we have a string. Otherwise, cast it
				// using fmt.Sprintf.
				if str, ok := sli[i].(string); ok {
					v = str
				} else {
					v = fmt.Sprintf("%v", sli[i])
				}

				// Add the string to the slice
				strSli = append(strSli, v)
			}
		default:
			return attribute.KeyValue{},
				fmt.Errorf("got unsupported type %q for %s", reflect.ValueOf(kv.Value.Value).Kind(), attribute.STRINGSLICE.String())
		}

		return attribute.StringSlice(kv.Key, strSli), nil
	default:
		return attribute.KeyValue{}, fmt.Errorf("unknown value type %s", kv.Value.Type)
	}
}

// traceIDFromHex returns a TraceID from a hex string if it is compliant with
// the W3C trace-context specification.  See more at
// https://www.w3.org/TR/trace-context/#trace-id
// our copy removes the validity check
func traceIDFromHex(h string) (trace.TraceID, error) {
	t := trace.TraceID{}
	if len(h) != 32 {
		return t, errors.New("unable to parse trace id")
	}

	if err := decodeHex(h, t[:]); err != nil {
		return t, err
	}

	return t, nil
}

// spanIDFromHex returns a SpanID from a hex string if it is compliant
// with the w3c trace-context specification.
// See more at https://www.w3.org/TR/trace-context/#parent-id
// our version remove the validity check
func spanIDFromHex(h string) (trace.SpanID, error) {
	s := trace.SpanID{}
	if len(h) != 16 {
		return s, errors.New("unable to parse span id of length: %d")
	}

	if err := decodeHex(h, s[:]); err != nil {
		return s, err
	}

	return s, nil
}

// decodeHex decodes hex in a manner compliant with OpenTelemetry
func decodeHex(h string, b []byte) error {
	for _, r := range h {
		switch {
		case 'a' <= r && r <= 'f':
			continue
		case '0' <= r && r <= '9':
			continue
		default:
			return errors.New("unable to parse hex id")
		}
	}

	decoded, err := hex.DecodeString(h)
	if err != nil {
		return err
	}

	copy(b, decoded)
	return nil
}

// Snapshot turns a Span into a ReadOnlySpan which is exportable by otel.
func (s *Span) Snapshot() tracesdk.ReadOnlySpan {
	return spanSnapshot{
		name:                 s.Name,
		spanContext:          s.SpanContext,
		parent:               s.Parent,
		spanKind:             s.SpanKind,
		startTime:            s.StartTime,
		endTime:              s.EndTime,
		attributes:           s.Attributes,
		events:               s.Events,
		links:                s.Links,
		status:               s.Status,
		droppedAttributes:    s.DroppedAttributes,
		droppedEvents:        s.DroppedEvents,
		droppedLinks:         s.DroppedLinks,
		childSpanCount:       s.ChildSpanCount,
		resource:             resource.NewSchemaless(s.Resource...),
		instrumentationScope: s.InstrumentationLibrary,
	}
}

// spanSnapshot is a helper type for transforming a Span into a ReadOnlySpan.
type spanSnapshot struct {
	// Embed the interface to implement the private method.
	tracesdk.ReadOnlySpan

	name                 string
	spanContext          trace.SpanContext
	parent               trace.SpanContext
	spanKind             trace.SpanKind
	startTime            time.Time
	endTime              time.Time
	attributes           []attribute.KeyValue
	events               []tracesdk.Event
	links                []tracesdk.Link
	status               tracesdk.Status
	droppedAttributes    int
	droppedEvents        int
	droppedLinks         int
	childSpanCount       int
	resource             *resource.Resource
	instrumentationScope instrumentation.Scope
}

// Name returns the Name of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Name() string { return s.name }

// SpanContext returns the SpanContext of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) SpanContext() trace.SpanContext { return s.spanContext }

// Parent returns the Parent of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Parent() trace.SpanContext { return s.parent }

// SpanKind returns the SpanKind of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) SpanKind() trace.SpanKind { return s.spanKind }

// StartTime returns the StartTime of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) StartTime() time.Time { return s.startTime }

// EndTime returns the EndTime of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) EndTime() time.Time { return s.endTime }

// Attributes returns the Attributes of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Attributes() []attribute.KeyValue { return s.attributes }

// Links returns the Links of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Links() []tracesdk.Link { return s.links }

// Events returns the Events of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Events() []tracesdk.Event { return s.events }

// Status returns the Status of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Status() tracesdk.Status { return s.status }

// DroppedAttributes returns the DroppedAttributes of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) DroppedAttributes() int { return s.droppedAttributes }

// DroppedLinks returns the DroppedLinks of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) DroppedLinks() int { return s.droppedLinks }

// DroppedEvents returns the DroppedEvents of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) DroppedEvents() int { return s.droppedEvents }

// ChildSpanCount returns the ChildSpanCount of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) ChildSpanCount() int { return s.childSpanCount }

// Resource returns the Resource of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Resource() *resource.Resource { return s.resource }

// InstrumentationScope returns the InstrumentationScope of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) InstrumentationScope() instrumentation.Scope {
	return s.instrumentationScope
}

// InstrumentationLibrary returns the InstrumentationLibrary of the snapshot
//
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) InstrumentationLibrary() instrumentation.Library {
	return s.instrumentationScope
}
