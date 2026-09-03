// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description: Tests for gRPC status and context error classification

package orerr_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
	"gotest.tools/v3/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestExtractErrorStatusCode_GRPCStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected statuscodes.StatusCode
		category statuscodes.StatusCategory
	}{
		{
			name:     "grpc NotFound direct",
			err:      status.Error(codes.NotFound, "not found"),
			expected: statuscodes.NotFound,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc NotFound wrapped",
			err:      fmt.Errorf("fetching thing: %w", status.Error(codes.NotFound, "not found")),
			expected: statuscodes.NotFound,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc InvalidArgument",
			err:      status.Error(codes.InvalidArgument, "bad input"),
			expected: statuscodes.BadRequest,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc Unauthenticated",
			err:      status.Error(codes.Unauthenticated, "no auth"),
			expected: statuscodes.Unauthorized,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc PermissionDenied",
			err:      status.Error(codes.PermissionDenied, "forbidden"),
			expected: statuscodes.Forbidden,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc ResourceExhausted",
			err:      status.Error(codes.ResourceExhausted, "rate limit"),
			expected: statuscodes.RateLimited,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc DeadlineExceeded",
			err:      status.Error(codes.DeadlineExceeded, "timeout"),
			expected: statuscodes.DeadlineExceeded,
			category: statuscodes.CategoryServerError,
		},
		{
			name:     "grpc Canceled",
			err:      status.Error(codes.Canceled, "canceled"),
			expected: statuscodes.Cancelled,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc Internal",
			err:      status.Error(codes.Internal, "server error"),
			expected: statuscodes.InternalServerError,
			category: statuscodes.CategoryServerError,
		},
		{
			name:     "grpc Unavailable",
			err:      status.Error(codes.Unavailable, "unavailable"),
			expected: statuscodes.Unavailable,
			category: statuscodes.CategoryServerError,
		},
		{
			name:     "grpc Unimplemented",
			err:      status.Error(codes.Unimplemented, "not implemented"),
			expected: statuscodes.NotImplemented,
			category: statuscodes.CategoryServerError,
		},
		{
			name:     "grpc AlreadyExists",
			err:      status.Error(codes.AlreadyExists, "conflict"),
			expected: statuscodes.Conflict,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "grpc Unknown unmapped",
			err:      status.Error(codes.Unknown, "unknown"),
			expected: statuscodes.InternalServerError,
			category: statuscodes.CategoryServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := orerr.ExtractErrorStatusCode(tt.err)
			assert.Equal(t, tt.expected, code, "status code mismatch")

			cat := orerr.ExtractErrorStatusCategory(tt.err)
			assert.Equal(t, tt.category, cat, "status category mismatch")
		})
	}
}

func TestExtractErrorStatusCode_ContextErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected statuscodes.StatusCode
		category statuscodes.StatusCategory
	}{
		{
			name:     "context.Canceled direct",
			err:      context.Canceled,
			expected: statuscodes.Cancelled,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "context.Canceled wrapped",
			err:      fmt.Errorf("request failed: %w", context.Canceled),
			expected: statuscodes.Cancelled,
			category: statuscodes.CategoryClientError,
		},
		{
			name:     "context.DeadlineExceeded direct",
			err:      context.DeadlineExceeded,
			expected: statuscodes.DeadlineExceeded,
			category: statuscodes.CategoryServerError,
		},
		{
			name:     "context.DeadlineExceeded wrapped",
			err:      fmt.Errorf("timed out: %w", context.DeadlineExceeded),
			expected: statuscodes.DeadlineExceeded,
			category: statuscodes.CategoryServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := orerr.ExtractErrorStatusCode(tt.err)
			assert.Equal(t, tt.expected, code, "status code mismatch")

			cat := orerr.ExtractErrorStatusCategory(tt.err)
			assert.Equal(t, tt.category, cat, "status category mismatch")
		})
	}
}

func TestExtractErrorStatusCode_StatusCodeWrapperPrecedence(t *testing.T) {
	grpcErr := status.Error(codes.NotFound, "not found")
	wrapped := orerr.NewErrorStatus(grpcErr, statuscodes.BadRequest)

	code := orerr.ExtractErrorStatusCode(wrapped)
	assert.Equal(t, statuscodes.BadRequest, code, "StatusCodeWrapper should take precedence")

	cat := orerr.ExtractErrorStatusCategory(wrapped)
	assert.Equal(t, statuscodes.CategoryClientError, cat)
}

func TestExtractErrorStatusCode_PlainError(t *testing.T) {
	err := fmt.Errorf("plain error")

	code := orerr.ExtractErrorStatusCode(err)
	assert.Equal(t, statuscodes.InternalServerError, code, "plain error should default to InternalServerError")

	cat := orerr.ExtractErrorStatusCategory(err)
	assert.Equal(t, statuscodes.CategoryServerError, cat)
}
