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
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
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

// Snapshots returns a slice of ReadOnlySpans exportable by otle.
func (t Trace) Snapshots() []tracesdk.ReadOnlySpan {
	var spans []tracesdk.ReadOnlySpan
	for _, span := range t.Spans {
		spans = append(spans, span.Snapshot())
	}
	return spans
}
