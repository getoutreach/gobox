//go:build !or_e2e

package orerr_test

import (
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
)

// Compile-time check that the wrapper satisfies the interface events looks for.
var _ events.KindError = (interface {
	error
	ErrorKind() string
})(nil)

// kindOf returns the nearest kind in err's chain as events.NewErrorInfo sees it.
func kindOf(err error) string {
	var ke events.KindError
	if errors.As(err, &ke) {
		return ke.ErrorKind()
	}
	return ""
}

func TestKind(t *testing.T) {
	base := errors.New("base")

	tests := []struct {
		name     string
		build    func() error
		wantNil  bool
		wantKind string
		wantIs   error // errors.Is target, if any
		wantMsg  string
	}{
		{name: "nil error", build: func() error { return orerr.Kind(nil, "k") }, wantNil: true},
		{name: "nil error via New", build: func() error { return orerr.New(nil, orerr.WithKind("k")) }, wantNil: true},
		{name: "empty kind is no-op", build: func() error { return orerr.Kind(base, "") }, wantKind: "", wantIs: base, wantMsg: "base"},
		{name: "empty kind via option is no-op", build: func() error { return orerr.New(base, orerr.WithKind("")) }, wantKind: "", wantIs: base, wantMsg: "base"},
		{name: "plain", build: func() error { return orerr.Kind(base, "k") }, wantKind: "k", wantIs: base, wantMsg: "base"},
		{name: "option", build: func() error { return orerr.New(base, orerr.WithKind("k")) }, wantKind: "k", wantIs: base, wantMsg: "base"},
		{
			name:     "wrapped by fmt.Errorf",
			build:    func() error { return fmt.Errorf("ctx: %w", orerr.Kind(base, "k")) },
			wantKind: "k", wantIs: base, wantMsg: "ctx: base",
		},
		{
			name:     "wrapping Info",
			build:    func() error { return orerr.Kind(orerr.Info(base, log.F{"a": 1}), "k") },
			wantKind: "k", wantIs: base, wantMsg: "base",
		},
		{
			name:     "wrapped by Info",
			build:    func() error { return orerr.Info(orerr.Kind(base, "k"), log.F{"a": 1}) },
			wantKind: "k", wantIs: base, wantMsg: "base",
		},
		{
			name:     "wrapped by Retryable",
			build:    func() error { return orerr.Retryable(orerr.Kind(base, "k")) },
			wantKind: "k", wantIs: base, wantMsg: "base",
		},
		{
			name:     "wrapping Retryable",
			build:    func() error { return orerr.Kind(orerr.Retryable(base), "k") },
			wantKind: "k", wantIs: base, wantMsg: "base",
		},
		{
			name: "all options",
			build: func() error {
				return orerr.New(base, orerr.WithInfo(log.F{"a": 1}), orerr.WithKind("k"), orerr.WithRetry())
			},
			wantKind: "k", wantIs: base, wantMsg: "base",
		},
		{
			name:     "outermost kind wins",
			build:    func() error { return orerr.Kind(fmt.Errorf("x: %w", orerr.Kind(base, "inner")), "outer") },
			wantKind: "outer", wantIs: base, wantMsg: "x: base",
		},
		{
			name:     "inner kind visible through plain wrapper",
			build:    func() error { return fmt.Errorf("x: %w", orerr.Kind(base, "inner")) },
			wantKind: "inner", wantIs: base, wantMsg: "x: base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.build()
			if tt.wantNil {
				assert.NilError(t, err)
				return
			}
			assert.Assert(t, err != nil)
			assert.Equal(t, err.Error(), tt.wantMsg)
			assert.Equal(t, kindOf(err), tt.wantKind)
			assert.Assert(t, errors.Is(err, tt.wantIs))
		})
	}
}

func TestKindUnwrap(t *testing.T) {
	base := errors.New("base")
	err := orerr.Kind(base, "k")
	assert.Equal(t, errors.Unwrap(err), base)
}

func TestKindPreservesRetryableAndInfo(t *testing.T) {
	base := errors.New("base")
	err := orerr.Kind(orerr.Retryable(orerr.Info(base, log.F{"a": 1})), "k")
	assert.Assert(t, orerr.IsRetryable(err))

	got := log.F{}
	m, ok := err.(log.Marshaler) //nolint:errorlint // Why: test
	assert.Assert(t, ok)
	m.MarshalLog(got.Set)
	assert.Equal(t, got["a"], 1)
}
