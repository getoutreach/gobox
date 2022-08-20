// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains an implementation of the CompressedReader interface
// for decompressing a xz file.

package archive

import (
	"context"
	"io"

	"github.com/ulikunitz/xz"
)

// _ ensures that the xzCompressedReader type implements the CompressedReader interface
var _ CompressedReader = &xzCompressedReader{}

// xzCompressedReader is a CompressedReader for xz compressed file(s)
type xzCompressedReader struct{}

// Open returns a reader for a xz file
func (x *xzCompressedReader) Open(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	xzr, err := xz.NewReader(r)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(xzr), nil
}
