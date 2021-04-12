package cfg

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/pkg/secrets"
)

// Secret provides access to secret config.
//
// The actual secret should be fetched via Data() which returns a SecretData.
//
// This type can be embedded within Config
type Secret struct {
	Path string `yaml:"Path"`
}

// Data fetches the data for this secret.
//
// Do not cache this value.  When passing this to internal outreach
// functions, pass the SecretData -- do not conver it to string first.
func (s Secret) Data(ctx context.Context) (SecretData, error) {
	str, err := secrets.Config(ctx, s.Path)
	return SecretData(str), err
}

// SecretData just wraps strings so we don't accidentally log secret
// data.
//
// Do not store SecretData -- it is only meant to be kept in scope
// variables and as arguments.
//
// Do not pass the raw converted string as arguments to any outreach
// code, use the strong type SecretData instead
type SecretData string

// MarshalJSON implements a dummy json.Marshaler
func (s SecretData) MarshalJSON() ([]byte, error) {
	return []byte("redacted"), nil
}

// MarshalYAML implements a dummy yaml.Marshaler
func (s SecretData) MarshalYAML() (interface{}, error) {
	return "redacted", nil
}

// GoString implements the GoStringer interface
func (s SecretData) GoString() string {
	return "redacted"
}

// Format implements fmt.Formatter
func (s SecretData) Format(f fmt.State, c rune) {
	if _, err := f.Write([]byte("redacted")); err != nil {
		panic(err)
	}
}
