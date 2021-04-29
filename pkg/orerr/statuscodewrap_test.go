package orerr_test

import (
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

func (suite) Basics(t *testing.T) {
	erro := errors.New("bad")
	err := orerr.NewErrorStatus(erro, statuscodes.Forbidden)
	assert.Equal(t, err.(*orerr.StatusCodeWrapper).StatusCode(), statuscodes.Forbidden)
	assert.Equal(t, err.(*orerr.StatusCodeWrapper).StatusCategory(), statuscodes.CategoryServerError)
	assert.Assert(t, orerr.IsErrorStatusCode(err, statuscodes.Forbidden))
	assert.Assert(t, orerr.IsErrorStatusCategory(err, statuscodes.CategoryClientError))
	assert.Assert(t, !orerr.IsErrorStatusCategory(err, statuscodes.CategoryServerError))
	assert.Assert(t, !orerr.IsErrorStatusCategory(err, statuscodes.CategoryOK))
}
