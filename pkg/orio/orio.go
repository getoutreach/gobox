// Package orio implements IO utilities.
package orio

import "io"

// ReadCloser provides a simple way to create an io.ReadCloser by
// combining io.Reader and io.Closer
type ReadCloser struct {
	io.Reader
	io.Closer
}

// WriteCloser provides a simple way to create an io.WriteCloser by
// combining io.Writer and io.Closer
type WriteCloser struct {
	io.Writer
	io.Closer
}

// Error is a io.ReadWriteClose implementation that returns the
// underlying error for everything.
type Error struct {
	Err error
}

func (e Error) Read(p []byte) (int, error) {
	return 0, e.Err
}

func (e Error) Write(p []byte) (int, error) {
	return 0, e.Err
}

func (e Error) Close() error {
	return e.Err
}
