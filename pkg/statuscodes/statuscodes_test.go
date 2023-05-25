package statuscodes

import "testing"

func TestStatusCodeUnmarshalText(t *testing.T) {
	codes := []StatusCode{
		OK,
		BadRequest,
		Unauthorized,
		Forbidden,
		NotFound,
		Conflict,
		RateLimited,
		InternalServerError,
		NotImplemented,
		Unavailable,
		UnknownError,
		DeadlineExceeded,
		Canceled,
	}

	for _, code := range codes {
		in := []byte(code.String())

		var out StatusCode
		if err := out.UnmarshalText(in); err != nil {
			t.Fatal(err)
		}

		if out != code {
			t.Fatalf("code was %v, expected %v", out, code)
		}
	}
}

func TestStatusCodeUnmarshalTextError(t *testing.T) {
	var out StatusCode
	if err := out.UnmarshalText([]byte("invalid")); err == nil {
		t.Fatal("error was nil")
	}
}
