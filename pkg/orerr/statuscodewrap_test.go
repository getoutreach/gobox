//go:build !or_e2e

package orerr_test

import (
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

func (suite) TestBasics(t *testing.T) {
	erro := errors.New("bad")
	err := orerr.NewErrorStatus(erro, statuscodes.Forbidden)

	//nolint:errorlint // Why: test
	assert.Equal(t, err.(*orerr.StatusCodeWrapper).StatusCode(), statuscodes.Forbidden)
	//nolint:errorlint // Why: test
	assert.Equal(t, err.(*orerr.StatusCodeWrapper).StatusCategory(), statuscodes.CategoryClientError)
	assert.Assert(t, orerr.IsErrorStatusCode(err, statuscodes.Forbidden))
	assert.Assert(t, orerr.IsErrorStatusCategory(err, statuscodes.CategoryClientError))
	assert.Assert(t, !orerr.IsErrorStatusCategory(err, statuscodes.CategoryServerError))
	assert.Assert(t, !orerr.IsErrorStatusCategory(err, statuscodes.CategoryOK))
}

func (suite) TestIs(t *testing.T) {
	unwrappedError := errors.New("bad")
	wrappedError := orerr.NewErrorStatus(unwrappedError, statuscodes.Forbidden)
	doubleWrappedError := errors.Wrap(wrappedError, "bad 2")

	assert.Assert(t, errors.Is(wrappedError, &orerr.StatusCodeWrapper{}))
	assert.Assert(t, errors.Is(doubleWrappedError, &orerr.StatusCodeWrapper{}))
	assert.Assert(t, !errors.Is(unwrappedError, &orerr.StatusCodeWrapper{}))
	assert.Assert(t, !errors.Is(nil, &orerr.StatusCodeWrapper{}))
}
