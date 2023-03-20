// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements a recorder which records
// all writes to it as frames in a log file. This also includes
// metadata about it.

package logfile

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
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

	// finish is a channel to signal when the recorder has should finish its work
	finish chan struct{}

	// listener is the network listener for the trace server
	listener net.Listener

	// finished is a channel to signal when all writing is complete
	finished chan struct{}
}

// newRecorder creates a new recorder using a os.File as
// the underlying writer
func newRecorder(logFile *os.File, width, height int, cmd string, args []string,
	l net.Listener) *recorder {
	enc := jsoniter.NewEncoder(logFile)
	startedAt := time.Now()

	//nolint:errcheck // Why: Best effort
	enc.Encode(NewMetadataEntry(startedAt, width, height, cmd, args))

	finish := make(chan struct{})
	finished := make(chan struct{})
	rec := &recorder{
		enc:       enc,
		startedAt: startedAt,
		lastWrite: startedAt,
		finish:    finish,
		listener:  l,
		finished:  finished,
	}

	rec.startTraceServer()

	return rec
}

// Write implements io.Writer by writing the data to the recorder
// in the form of frames
func (r *recorder) Write(b []byte) (n int, err error) {
	// Ensure that we only write one frame at a time
	r.mu.Lock()
	defer r.mu.Unlock()

	lastWriteTime := r.lastWrite
	r.lastWrite = time.Now()

	// use the time difference from the lastWriteTime, or use a
	// fixed diff if provided
	diff := time.Since(lastWriteTime)
	if r.fixedDiff != 0 {
		diff = r.fixedDiff
	}

	//nolint:errcheck // Why: We don't want failure to write to the log to cause the command to fail
	r.enc.Encode(NewFrameEntry(diff, b))
	return len(b), nil
}

// WriteTrace writes a trace to the recorder in the form of a frame
func (r *recorder) WriteTrace(reader io.Reader) error {
	// Decode the provided bytes into spans
	var spans []*Span
	for {
		if err := json.NewDecoder(reader).Decode(&spans); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("unable to unmarshal trace data: %w", err)
		}
	}

	// Ensure that we only write one frame at a time
	r.mu.Lock()
	defer r.mu.Unlock()

	//nolint:errcheck // Why: We don't want failure to write to the log to cause the command to fail
	r.enc.Encode(NewTraceEntry(spans))
	return nil
}

func (r *recorder) Shutdown() {
	// Signal the recorder that it needs to finish
	close(r.finish)

	// Wait for traces to flush
	<-r.finished
}

// startTraceServer creates a server that listens for traces on the default socket tand writes them to
// the provided recorder.
func (r *recorder) startTraceServer() {
	// terminate the listener when the command exits
	go func() {
		<-r.finish
		r.listener.Close()
	}()

	// start a server to listen for traces, closing finishedChan when the server exits
	go func() {
		defer close(r.finished)
		for {
			c, err := r.listener.Accept()
			// if the listener was closed, we're done and can return
			if errors.Is(err, net.ErrClosed) {
				return
			}
			r.handleConnection(c)
		}
	}()
}

// handleConnection reads from a connection and writes out
// to the traces to the provided recorder.
func (r *recorder) handleConnection(c net.Conn) {
	defer c.Close()
	if err := r.WriteTrace(c); err != nil {
		return
	}
}
