// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains options for the archive package.

package archive

// ExtractOptions are the options for the Extract function.
type ExtractOptions struct {
	// FilePath is the file path to extract out of the provided archive
	FilePath string

	// FilePathSelectorFunc is a function that can be used to select a file path
	// during extraction when there's a set criteria (or logic) that needs to be
	// ran to determine which file path to extract out of the archive.
	//
	// Note: When this function returns true, the file is returned. Multiple files
	// are not supported at this time.
	FilePathSelectorFunc func(string) bool
}

// ExtractOptionFunc is an option function that mutates an ExtractOptions struct.
type ExtractOptionFunc func(*ExtractOptions) error

// WithFilePath is an ExtractOptionFunc that sets the file path to extract out of
// the provided archive.
func WithFilePath(filePath string) ExtractOptionFunc {
	return func(opts *ExtractOptions) error {
		opts.FilePath = filePath
		return nil
	}
}

// WithFilePathSelector is an ExtractOptionFunc that sets the file path selector
// function. See ExtractOptions.FilePathSelectorFunc for more details.
func WithFilePathSelector(fn func(string) bool) ExtractOptionFunc {
	return func(opts *ExtractOptions) error {
		opts.FilePathSelectorFunc = fn
		return nil
	}
}
