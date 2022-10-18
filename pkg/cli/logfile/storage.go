// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the persistent storage format
// for logfiles.

package logfile

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// MetadataVersion is the version of the metadata format this
// package supports.
const MetadataVersion = 1

// FrameVersion is the version of frames this package supports
const FrameVersion = 1

// Metadata is the first entry in a log file that contains
// information about the log file.
type Metadata struct {
	// EntryMetadata implements a entry
	EntryMetadata `json:",inline"`

	// Version is the version of the metadata format
	Version int `json:"version"`

	// FrameVersion is the version of the frame format used
	FrameVersion int `json:"frame_version"`

	// Width is the width of the terminal
	Width int `json:"width"`

	// Height is the height of the terminal
	Height int `json:"height"`

	// StartedAt is the time that the process was started.
	StartedAt time.Time `json:"started_at"`

	// Command is the binary that was executed.
	Command string `json:"command"`

	// Args is the arguments that were passed to the binary.
	Args []string `json:"args"`
}

// Frame is a frame in a log file that contains the frames
// written to a terminal and the time between them.
type Frame struct {
	// EntryMetadata implements a entry
	EntryMetadata `json:",inline"`

	// Delay is the delay since the last frame.
	Delay time.Duration `json:"d"`

	// Bytes is the bytes written to the terminal.
	Bytes []byte `json:"b"`
}

// Trace is an entry in the logfile representing an otel trace.
type Trace struct {
	// EntryMetadata implements a entry
	EntryMetadata `json:",inline"`

	// Spans is a list of spans
	Spans []*Span `json:"spans"`
}

// ReadFromReader reads entires from a io.reader
func ReadFromReader(r io.Reader) ([]Entry, error) {
	return read(r)
}

// ReadFile reads a log file and returns the entries in it.
func ReadFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return read(f)
}

// read reads entries from a io.reader
func read(r io.Reader) ([]Entry, error) {
	var entries []Entry
	dec := json.NewDecoder(r)
	for {
		var fm Entry
		if err := dec.Decode(&fm); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		entries = append(entries, fm)
	}

	return entries, nil
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

// Snapshots returns a slice of ReadOnlySpans exportable by otle.
func (t Trace) Snapshots() []tracesdk.ReadOnlySpan {
	var spans []tracesdk.ReadOnlySpan
	for _, span := range t.Spans {
		spans = append(spans, span.Snapshot())
	}
	return spans
}
