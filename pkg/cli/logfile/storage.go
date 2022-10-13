// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the persistent storage format
// for logfiles.

package logfile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	// jsoniter is used to speed up json encoding for the log
	// paths, but not for playing
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

// FrameOrMetadata is a type that can be either a Frame or a Metadata
type FrameOrMetadata struct {
	Frame    *Frame
	Metadata *Metadata
}

// MarshalJSON implements json.Marshaler
func (f *FrameOrMetadata) MarshalJSON() ([]byte, error) {
	if f.Frame != nil {
		return jsoniter.Marshal(f.Frame)
	}
	return jsoniter.Marshal(f.Metadata)
}

// UnmarshalJSON implements json.Unmarshaler
func (f *FrameOrMetadata) UnmarshalJSON(data []byte) error {
	var frame Frame
	if err := jsoniter.Unmarshal(data, &frame); err == nil {
		f.Frame = &frame
		return nil
	}

	var metadata Metadata
	if err := jsoniter.Unmarshal(data, &metadata); err == nil {
		f.Metadata = &metadata
		return nil
	}

	return fmt.Errorf("unknown entry: %q", string(data))
}

// Metadata is the first entry in a log file that contains
// information about the log file.
type Metadata struct {
	// StartedAt is the time that the process was started.
	StartedAt time.Time `json:"started_at"`
}

// Frame is a frame in a log file that contains the frames
// written to a terminal and the time between them.
type Frame struct {
	// Delay is the delay since the last frame.
	Delay time.Duration `json:"d"`

	// Bytes is the bytes written to the terminal.
	Bytes []byte `json:"b"`
}

// ReadFile reads a log file and returns the frames and metadata.
func ReadFile(path string) ([]FrameOrMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return read(f)
}

// read reads frames from a io.reader
func read(r io.Reader) ([]FrameOrMetadata, error) {
	var frames []FrameOrMetadata
	dec := json.NewDecoder(r)
	for {
		var frame FrameOrMetadata
		if err := dec.Decode(&frame); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		frames = append(frames, frame)
	}

	return frames, nil
}
