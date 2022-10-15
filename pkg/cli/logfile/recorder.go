// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements a recorder which records
// all writes to it as frames in a log file. This also includes
// metadata about it.

package logfile

import (
	"io"
	"os"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// _ is a type assertion to ensure that recorder implements io.Writer
var _ io.Writer = (*recorder)(nil)

// recorder is a io.Writer that records all writes to it in the order
// they were written plus the time elapsed since the creation of the recorder
// and writes it to a json.Encoder
type recorder struct {
	mu sync.Mutex

	// enc is the encoder used to write the frames to the log file.
	enc *jsoniter.Encoder

	// startedAt is the time the recorder was created.
	startedAt time.Time

	// lastWrite is the time of the last write to the recorder.
	lastWrite time.Time

	// fixedDiff is a fixed time difference to use for testing
	fixedDiff time.Duration
}

// newRecorder creates a new recorder using a os.File as
// the underlying writer
func newRecorder(logFile *os.File, width, height int, cmd string, args []string) *recorder {
	enc := jsoniter.NewEncoder(logFile)
	startedAt := time.Now()

	//nolint:errcheck // Why: Best effort
	enc.Encode(NewMetadataEntry(startedAt, width, height, cmd, args))

	return &recorder{
		enc:       enc,
		startedAt: startedAt,
		lastWrite: startedAt,
	}
}

// Write implements io.Writer by writing the data to the recorder
// in the form of frames
func (r *recorder) Write(b []byte) (n int, err error) {
	var lastWriteTime time.Time

	// Capture the last write time under the lock and release
	// the lock as soon as possible
	r.mu.Lock()
	lastWriteTime = r.lastWrite
	r.lastWrite = time.Now()
	r.mu.Unlock()

	// use the time difference from the lastWriteTime, or use a
	// fixed diff if provided
	diff := time.Since(lastWriteTime)
	if r.fixedDiff != 0 {
		diff = r.fixedDiff
	}

	//nolint:errcheck // Why: Best effort
	r.enc.Encode(NewFrameEntry(diff, b))
	return len(b), nil
}
