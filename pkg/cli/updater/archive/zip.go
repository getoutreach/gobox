// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains an implementation of the Extractor
// and Archive interfaces for extracting a zip file.

package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"

	"github.com/pkg/errors"
)

// _ ensures that the zipExtractor type implements the Extractor interface
var _ Extractor = &zipExtractor{}

// _ ensures that the zipArchive type implements the Archive interface
var _ Archive = &zipArchive{}

// zipExtractor is an extractor for zip files
type zipExtractor struct{}

// Open returns a reader for the archive file.
//
// Note: Due to how zip files work, this function has to read the entire
// zip into memory before returning the Archive. It is recommended that gz/xz
// with tar be used instead when dealing with large zip files.
func (z *zipExtractor) Open(_ context.Context, _ string, r io.Reader) (Archive, error) {
	byt, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(byt), int64(len(byt)))
	if err != nil {
		return nil, err
	}

	return &zipArchive{zr, 0}, nil
}

// Close is a noop for the zipArchive type, implementing
// the io.Closer interface.
func (z *zipExtractor) Close() error {
	return nil
}

// zipArchive implements the Archive interface for zip files.
type zipArchive struct {
	r *zip.Reader

	pos int
}

// Next advances to the next file in the archive.
func (z *zipArchive) Next() (*Header, io.ReadCloser, error) {
	// if we've reached the end of the archive, return io.EOF
	if z.pos >= len(z.r.File) {
		return nil, nil, io.EOF
	}

	// get the next file in the archive
	// and increment the position in the array
	f := z.r.File[z.pos]
	z.pos++

	inf := f.FileInfo()
	h := &Header{
		Name: f.Name,
		Size: inf.Size(),
		Mode: int64(inf.Mode()),
		Type: HeaderTypeFile,
	}

	if inf.IsDir() {
		h.Type = HeaderTypeDirectory
	}

	r, err := f.Open()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create reader for file %q", f.Name)
	}

	return h, r, nil
}
