// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the persistent storage format
// for logfiles.

package logfile

import (
	"encoding/json"
	"io"
	"os"
	"time"

	// jsoniter is used to speed up json encoding for the log
	// paths, but not for playing

	"github.com/pkg/errors"
)

// Metadata is the first entry in a log file that contains
// information about the log file.
type Metadata struct {
	// EntryMetadata implements a entry
	EntryMetadata `json:",inline"`

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

// ReadFile reads a log file and returns the frames and metadata.
func ReadFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return read(f)
}

// read reads frames from a io.reader
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
