// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains an implementation of the CompressedReader interface
// for decompressing a bz2 file.

package archive

import (
	"compress/bzip2"
	"context"
	"io"
)

// _ ensures that the bz2CompressedReader type implements the CompressedReader interface
var _ CompressedReader = &bz2CompressedReader{}

// bz2CompressedReader is a CompressedReader for bz2 compressed file(s)
type bz2CompressedReader struct{}

// Open returns a reader for a bz2 file
func (g *bz2CompressedReader) Open(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(bzip2.NewReader(r)), nil
}
