// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file contains an implementation of the Extractor
// and Archive interfaces for extracting a zip file.
// converts to gzip then read as tar
// ref: https://stackoverflow.com/questions/57639648/how-to-decompress-tar-gz-file-in-go
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
)

// _ ensures that the tgsExtractor type implements the Extractor interface
var _ Extractor = &tgzExtractor{}

// tgzExtractor is an extractor for tgz files
type tgzExtractor struct{}

// _ ensures that the tgzArchive type implements the Archive interface
var _ Archive = &tgzArchive{}

// Open returns a reader for the archive file.
func (z *tgzExtractor) Open(ctx context.Context, name string, r io.Reader) (Archive, error) {
	byt, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	uncompressedStream, err := gzip.NewReader(bytes.NewReader(byt))
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(uncompressedStream)

	return &tgzArchive{tarReader}, nil
}

// Close is a noop for the tgzArchive type, implementing
// the io.Closer interface.
func (z *tgzExtractor) Close() error {
	return nil
}

// tgzArchive implements the Archive interface for tgz files.
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
