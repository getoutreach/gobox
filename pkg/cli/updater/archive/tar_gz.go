// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains an implementation of the CompressedReader interface
// for decompressing a gz file.

package archive

import (
	"compress/gzip"
	"context"
	"io"
)

// _ ensures that the gzipCompressedReader type implements the CompressedReader interface
var _ CompressedReader = &gzipCompressedReader{}

// gzipExtractor is a CompressedReader for gzipped compressed file(s)
type gzipCompressedReader struct{}

// Open returns a reader for a gzipped file
func (g *gzipCompressedReader) Open(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}
