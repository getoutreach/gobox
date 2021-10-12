package orio_test

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/orio"
	"gotest.tools/v3/assert"
)

func TestBufferedWriter(t *testing.T) {
	b := &orio.BufferedWriter{N: 5}

	n, err := b.Write([]byte("hell"))
	assert.Equal(t, n, 4)
	assert.NilError(t, err)
	assert.Equal(t, "hell", string(b.Bytes()))

	n, err = b.Write([]byte("hello world"))
	assert.Equal(t, n, 11)
	assert.NilError(t, err)
	assert.Equal(t, "world", string(b.Bytes()))

	n, err = b.Write([]byte("ly"))
	assert.Equal(t, n, 2)
	assert.NilError(t, err)
	assert.Equal(t, "rldly", string(b.Bytes()))
}
