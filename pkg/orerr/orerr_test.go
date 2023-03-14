//go:build !or_e2e

package orerr_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/shuffler"
)

func ExampleNew() {
	origErr := errors.New("something went wrong")
	info := log.F{"hello": "world"}
	err := orerr.New(origErr, orerr.WithInfo(info), orerr.WithRetry())

	formatted := log.F{}
	//nolint:errorlint // Why: test
	err.(log.Marshaler).MarshalLog(formatted.Set)
	fmt.Println("Err", err, orerr.IsRetryable(err), formatted)

	// Output: Err something went wrong true map[hello:world]
}

func ExampleCancelWithError() {
	origErr := errors.New("something went wrong")
	shutdownErr := &orerr.ShutdownError{Err: origErr}
	ctx, cancel := orerr.CancelWithError(context.Background())
	cancel(shutdownErr)

	fmt.Println("Err", ctx.Err())

	// Output: Err process has shutdown
}

func ExampleIsOneOf() {
	errList := []error{io.EOF, context.Canceled, context.DeadlineExceeded}

	if orerr.IsOneOf(io.EOF, errList...) {
		fmt.Println("io.EOF is part of the error list")
	}

	// Output:
	// io.EOF is part of the error list
}

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestNilNew(t *testing.T) {
	assert.Check(t, orerr.New(nil))
}

func (suite) TestRetryable(t *testing.T) {
	origErr := errors.New("something went wrong")
	err := orerr.Retryable(origErr)

	assert.Equal(t, origErr.Error(), err.Error())
	assert.Equal(t, origErr, errors.Unwrap(err))

	if !orerr.IsRetryable(err) {
		t.Fatal("failed IsRetryable check")
	}
}

func (suite) TestWithInfo(t *testing.T) {
	origErr := errors.New("something went wrong")
	info1 := log.F{"hello": "goodbye"}
	info2 := log.F{"foo": "bar"}

	err := orerr.Info(origErr, info1, info2)
	assert.Equal(t, origErr.Error(), err.Error())
	assert.Equal(t, origErr, errors.Unwrap(err))

	actual := log.F{}
	//nolint:errorlint // Why: test
	err.(log.Marshaler).MarshalLog(actual.Set)
	expected := log.F{"hello": "goodbye", "foo": "bar"}
	assert.DeepEqual(t, expected, actual)
}

func (suite) TestCancelWithError(t *testing.T) {
	err := errors.New("something went wrong")
	ctx, cancel := orerr.CancelWithError(context.Background())
	cancel(err)
	<-ctx.Done()
	assert.Equal(t, ctx.Err(), err)

	ctx, cancel = orerr.CancelWithError(context.Background())
	cancel(nil)
	<-ctx.Done()
	assert.Assert(t, errors.Is(ctx.Err(), context.Canceled))
}

func (suite) TestShutdownError(t *testing.T) {
	err := errors.New("something went wrong")
	shutdownErr := &orerr.ShutdownError{Err: err}
	assert.Assert(t, errors.Is(shutdownErr, err))
	assert.Equal(t, shutdownErr.Error(), "process has shutdown")
}

func (suite) TestLimitExceededError(t *testing.T) {
	err := errors.New("something went wrong")
	limitErr := &orerr.LimitExceededError{Kind: "queue", Err: err}
	assert.Assert(t, errors.Is(limitErr, err))
	assert.Equal(t, limitErr.Error(), "queue limit exceeded")
}

func (suite) TestExtractErrorMetadata(t *testing.T) {
	origErr := errors.New("something went wrong")
	meta := map[string]string{"my-service-status-code": "xxx"}

	err := orerr.Meta(origErr, meta)
	assert.DeepEqual(t, orerr.ExtractErrorMetadata(err), meta)
}

func (suite) TestExtractMetadataErrorEmpty(t *testing.T) {
	err := errors.New("something went wrong")
	assert.Equal(t, len(orerr.ExtractErrorMetadata(err)), 0)
}

func TestBadRequestError(t *testing.T) {
	err := errors.New("some input is wrong")
	badRequest := orerr.NewBadRequestError(err,
		orerr.NewViolation("length").WithDomain("prospect"),
	)
	assert.Assert(t, errors.Is(badRequest, err))
	assert.Equal(t, badRequest.Error(), "bad request: StatusCode: BadRequest, Wrapped: some input is wrong")
}
