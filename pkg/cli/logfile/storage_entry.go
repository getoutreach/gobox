// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the entry codec for the log file.

package logfile

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// EntryType is the type of entry in the log file
type EntryType int

const (
	// EntryTypeMetadata is a metadata entry which is equal to
	// a Metadata struct
	EntryTypeMetadata EntryType = iota

	// EntryTypeFrame is a frame entry which is equal to a Frame struct
	EntryTypeFrame

	// EntryTypeTrace is a trace entry representing a full or partial otel trace
	EntryTypeTrace
)

// Entry is an entry in the log file
type Entry struct {
	f *Frame
	m *Metadata
	t *Trace
}

// NewEntryFromFrame creates an entry from a frame
func NewEntryFromFrame(f *Frame) Entry {
	return Entry{
		f: f,
	}
}

// NewEntryFromMetadata creates an entry from metadata
func NewEntryFromMetadata(m *Metadata) Entry {
	return Entry{
		m: m,
	}
}

// NewEntryFromTrace creates an entry from a trace
func NewEntryFromTrace(t *Trace) Entry {
	return Entry{
		t: t,
	}
}

// NewFrameEntry creates a new frame entry
func NewFrameEntry(delay time.Duration, b []byte) Entry {
	return NewEntryFromFrame(&Frame{
		EntryMetadata: EntryMetadata{
			Type: EntryTypeFrame,
		},
		Delay: delay,
		Bytes: b,
	})
}

// NewMetadata creates a new metadata entry
func NewMetadataEntry(startedAt time.Time, width, height int, command string, args []string) Entry {
	return NewEntryFromMetadata(&Metadata{
		EntryMetadata: EntryMetadata{
			Type: EntryTypeMetadata,
		},
		Version:      MetadataVersion,
		FrameVersion: FrameVersion,
		StartedAt:    startedAt,
		Width:        width,
		Height:       height,
		Command:      command,
		Args:         args,
	})
}

// NewTraceEntry creates a new trace entry
func NewTraceEntry(spans []*Span) Entry {
	return NewEntryFromTrace(&Trace{
		EntryMetadata: EntryMetadata{
			Type: EntryTypeTrace,
		},
		Spans: spans,
	})
}

// MarshalJSON implements json.Marshaler for an entry
func (e Entry) MarshalJSON() ([]byte, error) {
	if e.IsFrame() {
		return jsoniter.Marshal(e.AsFrame())
	}

	if e.IsMetadata() {
		return jsoniter.Marshal(e.AsMetadata())
	}

	if e.IsTrace() {
		return jsoniter.Marshal(e.AsTrace())
	}

	return nil, fmt.Errorf("unknown entry type: %v", e)
}

// UnmarshalJSON implements json.Unmarshaler picking the correct
// type of entry based on the type field
func (e *Entry) UnmarshalJSON(data []byte) error {
	var em EntryMetadata
	if err := jsoniter.Unmarshal(data, &em); err != nil {
		return errors.Wrap(err, "unmarshaling entry metadata")
	}

	switch em.Type {
	case EntryTypeMetadata:
		e.m = &Metadata{}
		if err := jsoniter.Unmarshal(data, e.m); err != nil {
			return errors.Wrap(err, "unmarshaling metadata")
		}
	case EntryTypeFrame:
		e.f = &Frame{}
		if err := jsoniter.Unmarshal(data, e.f); err != nil {
			return errors.Wrap(err, "unmarshaling frame")
		}
	case EntryTypeTrace:
		e.t = &Trace{}
		if err := jsoniter.Unmarshal(data, e.t); err != nil {
			return errors.Wrap(err, "unmarshaling trace")
		}
	default:
		return fmt.Errorf("unknown entry type %v: '%s'", em.Type, string(data))
	}

	return nil
}

// IsFrame returns true if the entry is a frame
func (e Entry) IsFrame() bool {
	return e.f != nil
}

// IsMetadata returns true if the entry is metadata
func (e Entry) IsMetadata() bool {
	return e.m != nil
}

// IsTrace returns true if the entry is a trace
func (e Entry) IsTrace() bool {
	return e.t != nil
}

// AsMetadata returns the metadata from the current entry, or nil
// if it's not metadata
func (e Entry) AsMetadata() *Metadata {
	return e.m
}

// AsFrame returns the current frame or nil if it's not a frame
func (e Entry) AsFrame() *Frame {
	return e.f
}

// AsFrame returns the current frame or nil if it's not a frame
func (e Entry) AsTrace() *Trace {
	return e.t
}

// EntryMetadata is the basic metadata for an entry that must
// be present in all entries
type EntryMetadata struct {
	// Type is the type of entry in the log file
	Type EntryType `json:"t"`
}

// Span is a type similar to otel's SpanStub, but with the correct types needed
// for handle marshalling and unmarshalling.
type Span struct {
	Name              string
	SpanContext       trace.SpanContext
	Parent            trace.SpanContext
	SpanKind          trace.SpanKind
	StartTime         time.Time
	EndTime           time.Time
	Attributes        []attribute.KeyValue
	Events            []tracesdk.Event
	Links             []tracesdk.Link
	Status            tracesdk.Status
	DroppedAttributes int
	DroppedEvents     int
	DroppedLinks      int
	ChildSpanCount    int
	// We have to change this type from the otel type in order to make this struct marshallable
	Resource               []attribute.KeyValue
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
	case "INVALID":
		return attribute.KeyValue{}, errors.New("invalid value type")
	case "BOOL":
		return attribute.Bool(kv.Key, kv.Value.Value.(bool)), nil
	case "INT64":
		// JSON always decodes numbers to float64 so we have to manually re-cast
		return attribute.Int64(kv.Key, int64(kv.Value.Value.(float64))), nil
	case "FLOAT64":
		return attribute.Float64(kv.Key, kv.Value.Value.(float64)), nil
	case "STRING":
		return attribute.String(kv.Key, kv.Value.Value.(string)), nil
	case "BOOLSLICE":
		return attribute.BoolSlice(kv.Key, kv.Value.Value.([]bool)), nil
	case "INT64SLICE":
		// JSON always decodes numbers to float64 so we have to manually re-cast
		var v []int64
		for _, fv := range kv.Value.Value.([]float64) {
			v = append(v, int64(fv))
		}
		return attribute.Int64Slice(kv.Key, v), nil
	case "FLOAT64SLICE":
		return attribute.Float64Slice(kv.Key, kv.Value.Value.([]float64)), nil
	case "STRINGSLICE":
		return attribute.StringSlice(kv.Key, kv.Value.Value.([]string)), nil
	default:
		return attribute.KeyValue{}, errors.New("unsupported value type")
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
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Name() string { return s.name }

// SpanContext returns the SpanContext of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) SpanContext() trace.SpanContext { return s.spanContext }

// Parent returns the Parent of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Parent() trace.SpanContext { return s.parent }

// SpanKind returns the SpanKind of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) SpanKind() trace.SpanKind { return s.spanKind }

// StartTime returns the StartTime of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) StartTime() time.Time { return s.startTime }

// EndTime returns the EndTime of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) EndTime() time.Time { return s.endTime }

// Attributes returns the Attributes of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Attributes() []attribute.KeyValue { return s.attributes }

// Links returns the Links of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Links() []tracesdk.Link { return s.links }

// Events returns the Events of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Events() []tracesdk.Event { return s.events }

// Status returns the Status of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Status() tracesdk.Status { return s.status }

// DroppedAttributes returns the DroppedAttributes of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) DroppedAttributes() int { return s.droppedAttributes }

// DroppedLinks returns the DroppedLinks of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) DroppedLinks() int { return s.droppedLinks }

// DroppedEvents returns the DroppedEvents of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) DroppedEvents() int { return s.droppedEvents }

// ChildSpanCount returns the ChildSpanCount of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) ChildSpanCount() int { return s.childSpanCount }

// Resource returns the Resource of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) Resource() *resource.Resource { return s.resource }

// InstrumentationScope returns the InstrumentationScope of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) InstrumentationScope() instrumentation.Scope {
	return s.instrumentationScope
}

// InstrumentationLibrary returns the InstrumentationLibrary of the snapshot
//nolint:gocritic // Why: required by otel
func (s spanSnapshot) InstrumentationLibrary() instrumentation.Library {
	return s.instrumentationScope
}
