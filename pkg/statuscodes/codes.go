package statuscodes

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

//go:generate ../../scripts/gobin.sh golang.org/x/tools/cmd/stringer@v0.1.0 -type=StatusCode

type StatusCategory int

const (
	CategoryOK          StatusCategory = 1
	CategoryClientError StatusCategory = 2
	CategoryServerError StatusCategory = 3
)

//go:generate ../../scripts/gobin.sh golang.org/x/tools/cmd/stringer@v0.1.0 -type=StatusCategory

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
