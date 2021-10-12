package orio

// BufferedWriter buffers the last N bytes written to it.
//
// It does not error if more writes happen but uses a circular buffer
// and keeps only the last N bytes.
//
// Use Bytes() to access the buffer.
type BufferedWriter struct {
	buf []byte
	N   int
}

func (b *BufferedWriter) Write(p []byte) (int, error) {
	offset, size := 0, len(p)
	if bufSize := len(b.buf); size+bufSize >= b.N {
		if size >= b.N {
			offset = size - b.N
			b.buf = b.buf[:0]
		} else {
			b.buf = b.buf[bufSize-(b.N-size):]
		}
	}
	b.buf = append(b.buf, p[offset:]...)
	return size, nil
}

func (b *BufferedWriter) Bytes() []byte {
	return b.buf
}
