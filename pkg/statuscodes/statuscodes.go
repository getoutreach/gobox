// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements the statuscodes package.

// Package statuscodes is an attempt to create very-high-level buckets/classifications of errors, for two and ONLY two
// purposes:
//  1. Categories are intended for super-high-level bucketing of the responsibility for errors, ideally to be used for
//     SLOs/success rate metrics on dashboards/reporting (availability).
//  2. Codes are intended for high-level bucketing of categories of errors, so that generic framework-level http/grpc
//     clients can identify basic things like retriability, without understanding a ton of nuanced error codes.
//
// For sending service-specific errors, please wrap one of these errors into a more specific error type with your
// service-specific errors.  For example, bad_request.go has been added to this package, which wraps the basic
// BadRequest status code with more detailed information about the specific fields in question.
package statuscodes

import "fmt"

type StatusCode int

// Notes:
//  1. DO NOT EXTEND THIS LIST WITHOUT VERY CAREFUL CONSIDERATION.  Please read the package description at the top
//     of this file to understand the intent of these error codes (and categories).
//  2. Don't overlap with HTTP error codes so people know that these are not HTTP error codes when they see them in
//     server/service logs.
const (
	// Keep OK not as zero so you know someone affirmatively picked it
	OK StatusCode = 600

	// The 700-swath is for Client-caused error responses
	// BadRequest is for when the caller has provided input data that does not pass validation.  This is expected to
	// fail in all retry scenarios -- the caller needs to change the input data to succeed.
	BadRequest StatusCode = 700
	// Unauthorized is for when the caller has presented no or invalid credentials.
	Unauthorized StatusCode = 701
	// Forbidden is for when the caller has presented valid credentials to SOMETHING in the system, but they
	// specifically do not have access to the referred item.
	Forbidden StatusCode = 702
	// NotFound is for when the document attempting to be fetched or updated is not found.
	NotFound StatusCode = 703
	// Conflict should be used for when there is a conflict between the incoming data and the data existing in a
	// storage system (database, etc.), very similar to a BadRequest call (and it is similarly not expected to be
	// successfully retriable unless someone changes the data in the source system via another call).
	// Deprecated: In retrospect, this inclusion is a mistake compared to just having it be a nuance of BadRequest.
	Conflict StatusCode = 704
	// RateLimited is expected to be used when the client (or a set of clients) is/are sending too many requests that
	// are flooding the server.  It is expected that the client will back off for some duration and then try again.
	// Well-behaved services will even return an expected duration for the client to retry-after.
	RateLimited StatusCode = 705
	// ClientCanceled is for when the client has canceled the request, and the server has not yet started processing.
	// It is expected that sometimes the client will simply have a canceled request and no server request will have
	// been made.
	ClientCanceled StatusCode = 706

	// The 800-swath is for Server-caused error responses
	// InternalServerError is for when something otherwise uncategorizable has blown up inside the service logic.
	// This usually represents either a bug or an issue with a downstream system that the service is unable to
	// gracefully handle.  InternalServerErrors may or may not be retriable, it is unknown without more context.
	InternalServerError StatusCode = 800
	// NotImplemented is for when an endpoint exists and input appears valid, but the business logic behind it has
	// specifically not been implemented yet, but may exist in the future.  Without a further release of the service,
	// this error should not be retriable.
	NotImplemented StatusCode = 801
	// Unavailable is for when the server is experiencing a condition that is making all service temporarily
	// unavailable.  This error is potentially retriable, but the duration for backoff is unknown.
	Unavailable StatusCode = 802
	// UnknownError was intended to be a catchall error type for server-side issues.
	// Deprecated: In reality server-side errors should fall into one of the above 3 errors, and this inclusion was
	// a mistake.  It's not worth a breaking change to revoke at this time, though, so it shall live on.
	UnknownError StatusCode = 803
)

//go:generate ../../scripts/shell-wrapper.sh gobin.sh golang.org/x/tools/cmd/stringer@v0.1.12 -type=StatusCode

// StatusCodes map directly into StatusCategories by the numeric range of the status code.  See StatusCode comments
// for more details.
type StatusCategory int

const (
	CategoryOK          StatusCategory = 1
	CategoryClientError StatusCategory = 2
	CategoryServerError StatusCategory = 3
)

//go:generate ../../scripts/shell-wrapper.sh gobin.sh golang.org/x/tools/cmd/stringer@v0.1.12 -type=StatusCategory

func (re StatusCode) Category() StatusCategory {
	if re >= 600 && re <= 699 {
		return CategoryOK
	}

	if re >= 700 && re <= 799 {
		return CategoryClientError
	}

	if re >= 800 && re <= 899 {
		return CategoryServerError
	}

	return CategoryServerError
}

func (re *StatusCode) UnmarshalText(text []byte) error {
	code, ok := FromString(string(text))
	if !ok {
		return fmt.Errorf("invalid StatusCode '%s'", string(text))
	}

	*re = code
	return nil
}

func FromString(s string) (StatusCode, bool) {
	switch s {
	case OK.String():
		return OK, true
	case BadRequest.String():
		return BadRequest, true
	case Unauthorized.String():
		return Unauthorized, true
	case Forbidden.String():
		return Forbidden, true
	case NotFound.String():
		return NotFound, true
	case Conflict.String():
		return Conflict, true
	case RateLimited.String():
		return RateLimited, true
	case ClientCanceled.String():
		return ClientCanceled, true
	case InternalServerError.String():
		return InternalServerError, true
	case NotImplemented.String():
		return NotImplemented, true
	case Unavailable.String():
		return Unavailable, true
	case UnknownError.String():
		return UnknownError, true
	default:
		return UnknownError, false
	}
}
