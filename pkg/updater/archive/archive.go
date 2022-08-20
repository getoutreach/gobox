// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the main interface for extracting
// a certain file from an archive.

// Package archive contains methods for extracting file(s) from arbitrary archive
// types. The archive types supported are:
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
	if strings.Contains(archiveName, ".tar") {
		extractor = &tarExtractor{}
	} else if filepath.Ext(archiveName) == ".zip" {
		extractor = &zipExtractor{}
	}
	if extractor == nil {
		return nil, nil, fmt.Errorf("unsupported archive type: %v", archiveName)
	}
	defer extractor.Close()

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

		// If we have a FilePath, check if it matches. If it does
		// we found the file we want.
		if opts.FilePath != "" {
			if header.Name == opts.FilePath {
				// found the file
				return rc, header, nil
			}
		}

		// If we have a FilePathSelectorFunc we need to check if the file
		// matches the selector. If it does we found the file we want.
		if opts.FilePathSelectorFunc != nil {
			if opts.FilePathSelectorFunc(header.Name) {
				// found the file
				return rc, header, nil
			}
		}
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	return nil, nil, fmt.Errorf("no matching file in archive")
}
