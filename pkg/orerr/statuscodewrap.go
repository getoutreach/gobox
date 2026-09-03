// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements custom error for StatusCodeWrapper

package orerr

import (
	"context"
	"errors"

	"github.com/getoutreach/gobox/pkg/statuscodes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// Is implements the `errors.Is` interface to check whether an error is wrapped by `StatusCodeWrapper`.
func (w *StatusCodeWrapper) Is(target error) bool {
	_, ok := target.(*StatusCodeWrapper)
	return ok
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

	if errors.Is(err, context.Canceled) {
		return statuscodes.Cancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return statuscodes.DeadlineExceeded
	}

	var grpcStatus interface{ GRPCStatus() *status.Status }
	if errors.As(err, &grpcStatus) {
		if st := grpcStatus.GRPCStatus(); st != nil {
			return grpcCodeToStatusCode(st.Code())
		}
	}

	return statuscodes.InternalServerError
}

func ExtractErrorStatusCategory(err error) statuscodes.StatusCategory {
	code := ExtractErrorStatusCode(err)
	return code.Category()
}

func grpcCodeToStatusCode(code codes.Code) statuscodes.StatusCode {
	switch code {
	case codes.OK:
		return statuscodes.OK
	case codes.InvalidArgument:
		return statuscodes.BadRequest
	case codes.Unauthenticated:
		return statuscodes.Unauthorized
	case codes.PermissionDenied:
		return statuscodes.Forbidden
	case codes.NotFound:
		return statuscodes.NotFound
	case codes.ResourceExhausted:
		return statuscodes.RateLimited
	case codes.Internal:
		return statuscodes.InternalServerError
	case codes.Unimplemented:
		return statuscodes.NotImplemented
	case codes.Unavailable:
		return statuscodes.Unavailable
	case codes.DeadlineExceeded:
		return statuscodes.DeadlineExceeded
	case codes.Canceled:
		return statuscodes.Cancelled
	case codes.AlreadyExists:
		return statuscodes.Conflict
	case codes.FailedPrecondition, codes.Aborted:
		return statuscodes.Conflict
	case codes.OutOfRange:
		return statuscodes.BadRequest
	case codes.Unknown:
		return statuscodes.UnknownError
	case codes.DataLoss:
		return statuscodes.InternalServerError
	default:
		return statuscodes.InternalServerError
	}
}
