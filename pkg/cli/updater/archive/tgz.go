// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file contains an implementation of the Extractor
// and Archive interfaces for extracting a zip file.

package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
)

// _ ensures that the zipExtractor type implements the Extractor interface
var _ Extractor = &tgzExtractor{}

// zipExtractor is an extractor for zip files
type tgzExtractor struct{}

// _ ensures that the tgzArchive type implements the Archive interface
var _ Archive = &tgzArchive{}

// Open returns a reader for the archive file.
//
// Note: Due to how zip files work, this function has to read the entire
// zip into memory before returning the Archive. It is recommended that gz/xz
// with tar be used instead when dealing with large zip files.
func (z *tgzExtractor) Open(ctx context.Context, name string, r io.Reader) (Archive, error) {
	byt, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(byt), int64(len(byt)))
	if err != nil {
		return nil, err
	}

	return &tgzArchive{tr}, nil
}

// Close is a noop for the tgzArchive type, implementing
// the io.Closer interface.
func (z *tgzExtractor) Close() error {
	return nil
}

// tgzArchive implements the Archive interface for zip files.
type tgzArchive struct {
	r *tar.Reader
}

// Next advances to the next file in the archive.
func (t *tgzArchive) Next() (*Header, io.ReadCloser, error) {
	th, err := t.r.Next()
	if err != nil {
		return nil, nil, err
	}

	h := &Header{
		Name: th.Name,
		Size: th.Size,
		Mode: th.Mode,
	}

	if th.Typeflag == tar.TypeReg {
		h.Type = HeaderTypeFile
	} else if th.Typeflag == tar.TypeDir {
		h.Type = HeaderTypeDirectory
	}

	return h, io.NopCloser(t.r), nil
}
