package orerr

import (
	"errors"

	"github.com/getoutreach/gobox/pkg/statuscodes"
)

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
	var scw *StatusCodeWrapper
	if errors.As(err, &scw) {
		return scw.code == code
	}
	return false
}

func IsErrorStatusCategory(err error, category statuscodes.StatusCategory) bool {
	var scw *StatusCodeWrapper
	if errors.As(err, &scw) {
		return scw.StatusCategory() == category
	}
	return false
}

func ExtractErrorStatusCode(err error) statuscodes.StatusCode {
	var scw *StatusCodeWrapper
	if errors.As(err, &scw) {
		return scw.StatusCode()
	}
	return statuscodes.UnknownError
}

func ExtractErrorStatusCategory(err error) statuscodes.StatusCategory {
	var scw *StatusCodeWrapper
	if errors.As(err, &scw) {
		return scw.StatusCategory()
	}
	return statuscodes.CategoryServerError
}
