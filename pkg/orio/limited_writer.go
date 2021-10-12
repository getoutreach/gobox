package orio

import "errors"

// ErrLimitExceeded is returned if writes exceed the capacity.
var ErrLimitExceeded = errors.New("size limit exceeded")

// LimitedWriter limits write to a max of specified number of bytes.
//
// It returns ErrLimitExceeded if the write exceeds this limit.
type LimitedWriter struct {
	buf []byte
	N   int
}

func (l *LimitedWriter) Write(p []byte) (int, error) {
	var err error
	size := len(p)
	if size+len(l.buf) > l.N {
		size = l.N - len(l.buf)
		err = ErrLimitExceeded
	}

	l.buf = append(l.buf, p[:size]...)
	return size, err
}

func (l *LimitedWriter) Bytes() []byte {
	return l.buf
}
