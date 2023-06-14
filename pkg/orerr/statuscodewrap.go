// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements custom error for StatusCodeWrapper

package orerr

import (
	"errors"

	"github.com/getoutreach/gobox/pkg/statuscodes"
)

type StatusCodeProvider interface {
	StatusCode() statuscodes.StatusCode
}

type StatusCategoryProvider interface {
	StatusCategory() statuscodes.StatusCategory
}

type StatusCodeWrapper struct {
	wrappedErr error
	code       statuscodes.StatusCode
}

func (w *StatusCodeWrapper) Error() string {
	return "StatusCode: " + w.code.String() + ", Wrapped: " + w.wrappedErr.Error()
}

func (w *StatusCodeWrapper) StatusCode() statuscodes.StatusCode {
	return w.code
}

func (w *StatusCodeWrapper) StatusCategory() statuscodes.StatusCategory {
	return w.code.Category()
}

func (w *StatusCodeWrapper) Unwrap() error {
	return w.wrappedErr
}

func NewErrorStatus(errToWrap error, errCode statuscodes.StatusCode) error {
	return &StatusCodeWrapper{wrappedErr: errToWrap, code: errCode}
}

func IsErrorStatusCode(err error, code statuscodes.StatusCode) bool {
	var scp StatusCodeProvider
	if errors.As(err, &scp) {
		return scp.StatusCode() == code
	}
	return false
}

func IsErrorStatusCategory(err error, category statuscodes.StatusCategory) bool {
	var scp StatusCategoryProvider
	if errors.As(err, &scp) {
		return scp.StatusCategory() == category
	}
	return false
}

func ExtractErrorStatusCode(err error) statuscodes.StatusCode {
	var scp StatusCodeProvider
	if errors.As(err, &scp) {
		return scp.StatusCode()
	}
	return statuscodes.UnknownError
}

func ExtractErrorStatusCategory(err error) statuscodes.StatusCategory {
	var scp StatusCategoryProvider
	if errors.As(err, &scp) {
		return scp.StatusCategory()
	}
	return statuscodes.CategoryServerError
}
