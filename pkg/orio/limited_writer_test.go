package orio_test

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/orio"
	"gotest.tools/v3/assert"
)

func TestLimitedWriter(t *testing.T) {
	b := &orio.LimitedWriter{N: 10}

	n, err := b.Write([]byte("hello"))
	assert.Equal(t, n, 5)
	assert.NilError(t, err)
	assert.Equal(t, "hello", string(b.Bytes()))

	n, err = b.Write([]byte("world"))
	assert.Equal(t, n, 5)
	assert.NilError(t, err)
	assert.Equal(t, "helloworld", string(b.Bytes()))

	n, err = b.Write([]byte("foo"))
	assert.Equal(t, n, 0)
	assert.Equal(t, err, orio.ErrLimitExceeded)
}
