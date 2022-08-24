// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the main interface for extracting
// a certain file from an archive.

package archive

import (
	"context"
	"io"
)

// Extractor is the interface for creating a io.ReadCloser from
// an archive.
type Extractor interface {
	// Open returns a reader for the archive file
	Open(ctx context.Context, archiveName string, archive io.Reader) (Archive, error)

	// Close closes the reader returned by Open
	Close() error
}

// Header is a generic struct containing information about a file
// in an archive.
type Header struct {
	// Name is the name of the file entry
	Name string

	// Mode is the mode of the file entry
	Mode int64

	// Size is the size of the file entry
	Size int64

	// Type is the type of entry this is
	// (file, directory, etc)
	Type HeaderType
}

// HeaderType is the type of entry a Header is for in an archive
type HeaderType string

const (
	// HeaderTypeFile is a file entry
	HeaderTypeFile HeaderType = "file"

	// HeaderTypeDirectory is a directory entry
	HeaderTypeDirectory HeaderType = "directory"
)

// Archive is an interface for interacting with files inside of an archive
type Archive interface {
	// Next returns the next file in the archive, or returns
	// io.EOF if there are no more files.
	Next() (*Header, io.ReadCloser, error)
}
