package orio_test

import (
	"errors"
	"io"
	"testing"

	"github.com/getoutreach/gobox/pkg/orio"
	"gotest.tools/v3/assert"
)

var _ io.ReadWriteCloser = orio.Error{}

func TestError(t *testing.T) {
	e := orio.Error{Err: errors.New("foo")}

	n, err := e.Read(make([]byte, 10))
	assert.Equal(t, n, 0)
	assert.Equal(t, err, e.Err)

	n, err = e.Write(nil)
	assert.Equal(t, n, 0)
	assert.Equal(t, err, e.Err)

	assert.Equal(t, e.Close(), err)
}
