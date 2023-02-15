// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains code for interacting with tar files.

package archive

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"
)

// _ ensures that the tarExtractor type implements the Extractor interface
var _ Extractor = &tarExtractor{}

// _ ensures that the tarArchive type implements the Archive interface
var _ Archive = &tarArchive{}

// CompressedReader is the interface for a reader that can read a compressed
// file.
type CompressedReader interface {
	// Open returns a reader for the compressed file.
	// This only returns a io.ReadCloser because it should only contain a single file
	Open(ctx context.Context, r io.Reader) (io.ReadCloser, error)
}

// tarExtractor is an extractor for tar files
type tarExtractor struct {
	r io.ReadCloser
}

// Open returns a reader for the archive file
func (t *tarExtractor) Open(ctx context.Context, name string, r io.Reader) (Archive, error) {
	ext := filepath.Ext(name)

	var container CompressedReader

	switch ext {
	case ".tar":
		tr := tar.NewReader(r)
		t.r = io.NopCloser(tr)
		return &tarArchive{tr}, nil
	case ".gz":
		container = &gzipCompressedReader{}
	case ".tgz":
		container = &gzipCompressedReader{}
	case ".xz":
		container = &xzCompressedReader{}
	case ".bz2":
		container = &bz2CompressedReader{}
	default:
		return nil, fmt.Errorf("unsupported container type: %v", ext)
	}

	containerR, err := container.Open(ctx, r)
	if err != nil {
		return nil, err
	}

	tr := tar.NewReader(containerR)

	// Ensure we close in the order of tar -> container (compressed data)
	t.r = newSequencedReadCloser(io.NopCloser(tr), containerR)

	// return a new tarArchive that implements Archive with our tar.Reader
	return &tarArchive{tr}, nil
}

// Close closes the reader returned by Open. This can be called multiple
// times and when Open hasn't been called safely.
// This is not go-routine safe.
func (t *tarExtractor) Close() error {
	if t.r != nil {
		return t.r.Close()
	}

	return nil
}

// tarArchive implements the Archive interface for tar files.
type tarArchive struct {
	r *tar.Reader
}

// Next advances to the next file in the archive.
func (t *tarArchive) Next() (*Header, io.ReadCloser, error) {
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
