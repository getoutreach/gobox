package codec_test

import (
	"strings"
	"testing"

	"github.com/getoutreach/gobox/pkg/codec"
	"github.com/getoutreach/gobox/pkg/log"
	"gotest.tools/v3/assert"
)

func TestJSONDecoder(t *testing.T) {
	// Attempt to decode an invalid json.
	json := &codec.JSON{MaxSnippetSize: 3}
	var v interface{}
	err := json.NewDecoder(strings.NewReader("          foo")).Decode(&v)

	// Confirm that the error has a error.json payload.
	m, ok := err.(log.Marshaler) //nolint:errorlint // Why: test
	assert.Assert(t, ok && m != nil)
	fields := log.F{}
	m.MarshalLog(fields.Set)
	assert.DeepEqual(t, fields, log.F{"error.json": "foo"})
}
