// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains io related functions for the archive package.

package archive

import "io"

// sequencedReadCloser is a ReadCloser that closes all other closers it
// contains in the order they were added when Close is called.
// The first provided reader is embedded and implements the io.ReadCloser,
// minus the Close() method which is implemented by the sequencedCloser.
type sequencedReadCloser struct {
	io.ReadCloser
	rcs []io.Closer
}

// Close closes all of the contained ReadClosers in the order they were added
// to the sequencedCloser. If a reader fails to close, an error is returned
// and the rest are NOT closed.
func (n *sequencedReadCloser) Close() error {
	for _, c := range n.rcs {
		if err := c.Close(); err != nil {
			return err
		}
	}

	return nil
}

// newSequencedReadCloser returns a new sequencedReadCloser
func newSequencedReadCloser(rc io.ReadCloser, closers ...io.Closer) *sequencedReadCloser {
	return &sequencedReadCloser{rc, closers}
}
