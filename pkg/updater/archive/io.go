// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains io related functions for the archive package.

package archive

import "io"

// sequencedReadCloser is a reader that closes all other readers it contains
// in the order they were added. The first provided reader is embedded
// and implements the io.ReadCloser, minus the Close() method which is
// implemented by the sequencedCloser.
type sequencedReadCloser struct {
	io.ReadCloser
	closers []io.ReadCloser
}

// Close closes all of the contained readers in the order they were added
// to the sequencedCloser. If a reader fails to close, an error is returned
// and the rest are NOT closed.
func (n *sequencedReadCloser) Close() error {
	for _, c := range n.closers {
		if err := c.Close(); err != nil {
			return err
		}
	}

	return nil
}

// newSequencedReadCloser returns a new sequencedReadCloser
func newSequencedReadCloser(closers ...io.ReadCloser) *sequencedReadCloser {
	return &sequencedReadCloser{closers[0], closers}
}
