// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the main interface for extracting
// a certain file from an archive.

// Package archive contains methods for extracting file(s) from arbitrary archive
// types. The archive types supported are:
//   - tar
//   - tar.gz
//   - tar.xz
//   - tar.bz2
//   - zip
package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// IDEA(jaredallard): We could potentially refactor this to support mime-types,
// but that seems like a pretty big undertaking right now for not much benefit.
// Ref: https://github.com/gabriel-vasile/mimetype/blob/master/example_reader_test.go

// Extract extracts a file, based on the provided option functions, from
// the provided archive.
func Extract(ctx context.Context, archiveName string, r io.Reader,
	optFns ...ExtractOptionFunc) (io.ReadCloser, *Header, error) {
	opts := &ExtractOptions{}
	for _, fn := range optFns {
		if err := fn(opts); err != nil {
			return nil, nil, err
		}
	}
	if opts.FilePath == "" && opts.FilePathSelectorFunc == nil {
		return nil, nil, fmt.Errorf("WithFilePath or WithFilePathSelectorFunc must be provided via the options")
	}

	var extractor Extractor
  //nolint:gocritic // Why: we are checking if file extension contains a substring
	if strings.Contains(archiveName, ".tar") || filepath.Ext(archiveName) == ".tgz" {
		extractor = &tarExtractor{}
	} else if filepath.Ext(archiveName) == ".zip" {
		extractor = &zipExtractor{}
	}
	if extractor == nil {
		return nil, nil, fmt.Errorf("unsupported archive type: %v", archiveName)
	}

	// create an Archive interface from the archive
	archive, err := extractor.Open(ctx, archiveName, r)
	if err != nil {
		return nil, nil, err
	}

	for ctx.Err() == nil {
		header, rc, err := archive.Next()
		if errors.Is(err, io.EOF) {
			// Didn't find the file, break
			break
		} else if err != nil {
			return nil, nil, nil
		}

		// skip non-regular files
		if header.Type != HeaderTypeFile {
			continue
		}

		// if we found the file we want, return it
		if selectFile(header, opts) {
			// close the file then close the extractor
			return newSequencedReadCloser(rc, extractor), header, nil
		}
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	return nil, nil, fmt.Errorf("no matching file in archive")
}

// selectFile selects a file from the archive based on the provided options.
func selectFile(h *Header, opts *ExtractOptions) bool {
	// If we have a FilePath, check if it matches. If it does
	// we found the file we want.
	if opts.FilePath != "" {
		return h.Name == opts.FilePath
	}

	// If we have a FilePathSelectorFunc we need to check if the file
	// matches the selector. If it does we found the file we want.
	if opts.FilePathSelectorFunc != nil {
		return opts.FilePathSelectorFunc(h.Name)
	}

	return false
}
