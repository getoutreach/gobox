// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides custom status types and helpers

package statuscodes

import "fmt"

type StatusCode int

// 1. Keep OK not as zero so you know someone affirmatively picked it
// 2. Don't overlap with HTTP error codes so people know that these are different
const (
	OK StatusCode = 600

	// Client-caused error responses
	BadRequest   StatusCode = 700
	Unauthorized StatusCode = 701
	Forbidden    StatusCode = 702
	NotFound     StatusCode = 703
	Conflict     StatusCode = 704
	RateLimited  StatusCode = 705

	// Server-caused error responses
	InternalServerError StatusCode = 800
	NotImplemented      StatusCode = 801
	Unavailable         StatusCode = 802
	UnknownError        StatusCode = 803
)

//go:generate ../../scripts/shell-wrapper.sh gobin.sh golang.org/x/tools/cmd/stringer@v0.1.12 -type=StatusCode

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
