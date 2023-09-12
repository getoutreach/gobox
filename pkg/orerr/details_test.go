// Copyright 2023 Outreach Corporation. All Rights Reserved.

//go:build or_dev || or_test || or_e2e
// +build or_dev or_test or_e2e

package orerr_test

import (
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

func TestErrDetails(t *testing.T) {
	err := errors.New("some error")
	errDetails := orerr.NewErrDetails(err,
		orerr.NewErrDetail(
			"validationError",
			"Validation Error",
			"Custom1 must be one of these values: \"xxx\", \"yyy\"",
		),
	)
	assert.Assert(t, errors.Is(errDetails, err))
	assert.Equal(
		t,
		errDetails.Error(),
		"Details: [Validation Error(validationError): Custom1 must be one of these values: \"xxx\", \"yyy\"], Wrapped: some error",
	)
}

func TestNewErrDetails(t *testing.T) {
	err := orerr.NewErrDetails(errors.New("err"), orerr.NewErrDetail("id", "title", "detail"))

	assert.Equal(t, err.Error(), "Details: [title(id): detail], Wrapped: err")

	//nolint:errorlint // Why: test
	assert.Equal(t, err.(*orerr.ErrDetails).Details[0].ID, "id")
	//nolint:errorlint // Why: test
	assert.Equal(t, err.(*orerr.ErrDetails).Details[0].Title, "title")
	//nolint:errorlint // Why: test
	assert.Equal(t, err.(*orerr.ErrDetails).Details[0].Detail, "detail")
}

func TestWithDetails(t *testing.T) {
	err := orerr.New(errors.New("err"), orerr.WithDetails(orerr.NewErrDetail("id", "title", "detail")))
	assert.Equal(t, err.Error(), "Details: [title(id): detail], Wrapped: err")
}

func TestWithSourcePointer(t *testing.T) {
	code := "code"
	detail := orerr.NewErrDetail("id", "title", "detail").WithSourcePointer("pointer").WithCode(&code)

	err := orerr.New(errors.New("err"), orerr.WithDetails(detail))
	assert.Equal(t, err.Error(), "Details: [title(id): detail], Wrapped: err")
}

func TestWithDetailsAndStatus(t *testing.T) {
	err := orerr.New(
		errors.New("err"),
		orerr.WithDetails(orerr.NewErrDetail("id", "title", "detail")),
		orerr.WithStatus(statuscodes.Forbidden),
	)
	assert.Equal(t, err.Error(), "StatusCode: Forbidden, Wrapped: Details: [title(id): detail], Wrapped: err")
}
