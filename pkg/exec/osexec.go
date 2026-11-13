// Copyright 2026 Outreach Corporation. All Rights Reserved.
//
// Description: Code adapted from os/exec, so that Cmd behaves as os/exec.Cmd does.

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
)

// watchCtx calls cancel if ctx is done before Wait reads from result, and
// sends the error for Wait to result. It copies os/exec.Cmd.watchCtx without
// WaitDelay.
func watchCtx(ctx context.Context, cancel func() error, result chan<- error) {
	select {
	case result <- nil:
		return
	case <-ctx.Done():
	}

	var err error
	if cancelErr := cancel(); cancelErr == nil {
		// The command was interrupted, so what it does next can be due to ctx.
		err = ctx.Err()
	} else if !errors.Is(cancelErr, os.ErrProcessDone) {
		err = fmt.Errorf("exec: canceling Cmd: %w", cancelErr)
	}

	result <- err
}

// prefixSuffixSaver keeps the first and the last N bytes written to it. It
// has the same behavior as the type with the same name in os/exec, which
// Output uses to put the standard error on an *ExitError.
type prefixSuffixSaver struct {
	N         int // The maximum size of the prefix and of the suffix.
	prefix    []byte
	suffix    []byte // A ring buffer once len(suffix) == N.
	suffixOff int    // The offset in suffix of the next write.
	skipped   int64
}

// Write saves the parts of p that are in the first or last N bytes.
func (w *prefixSuffixSaver) Write(p []byte) (int, error) {
	lenp := len(p)
	p = w.fill(&w.prefix, p)

	// Keep only the last N bytes for the suffix.
	if overage := len(p) - w.N; overage > 0 {
		p = p[overage:]
		w.skipped += int64(overage)
	}
	p = w.fill(&w.suffix, p)

	// If p is not empty, suffix is full. Write over its oldest bytes.
	for len(p) > 0 {
		n := copy(w.suffix[w.suffixOff:], p)
		p = p[n:]
		w.skipped += int64(n)
		w.suffixOff += n
		if w.suffixOff == w.N {
			w.suffixOff = 0
		}
	}

	return lenp, nil
}

// Bytes returns the prefix and the suffix. If bytes were skipped between
// them, a line that tells how many separates the two.
func (w *prefixSuffixSaver) Bytes() []byte {
	if w.suffix == nil {
		return w.prefix
	}
	if w.skipped == 0 {
		return append(w.prefix, w.suffix...)
	}

	var buf bytes.Buffer
	buf.Grow(len(w.prefix) + len(w.suffix) + 50)
	buf.Write(w.prefix)
	buf.WriteString("\n... omitting ")
	buf.WriteString(strconv.FormatInt(w.skipped, 10))
	buf.WriteString(" bytes ...\n")
	buf.Write(w.suffix[w.suffixOff:])
	buf.Write(w.suffix[:w.suffixOff])

	return buf.Bytes()
}

// fill appends the start of p to *dst, until *dst has N bytes. It returns
// the rest of p.
func (w *prefixSuffixSaver) fill(dst *[]byte, p []byte) []byte {
	if remain := w.N - len(*dst); remain > 0 {
		add := min(len(p), remain)
		*dst = append(*dst, p[:add]...)
		p = p[add:]
	}
	return p
}
