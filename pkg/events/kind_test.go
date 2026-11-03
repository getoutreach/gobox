//go:build !or_e2e

package events_test

import (
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
)

// marshalEntries collects every addField call, preserving duplicates.
func marshalEntries(m log.Marshaler) [][2]interface{} {
	var out [][2]interface{}
	m.MarshalLog(func(k string, v interface{}) { out = append(out, [2]interface{}{k, v}) })
	return out
}

func kindValues(entries [][2]interface{}) []interface{} {
	var kinds []interface{}
	for _, e := range entries {
		if e[0] == "error.kind" {
			kinds = append(kinds, e[1])
		}
	}
	return kinds
}

type kindMarshalerError struct{}

func (kindMarshalerError) Error() string { return "marshaler error" }
func (kindMarshalerError) MarshalLog(addField func(string, interface{})) {
	addField("error.kind", "from-marshaler")
	addField("extra", "field")
}

func TestErrorInfoKind(t *testing.T) {
	base := errors.New("base")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no kind", base, "error"},
		{"no kind, wrapped", fmt.Errorf("ctx: %w", base), "error"},
		{"empty kind", orerr.Kind(base, ""), "error"},
		{"kind", orerr.Kind(base, "timeout"), "timeout"},
		{"kind wrapped by fmt.Errorf", fmt.Errorf("ctx: %w", orerr.Kind(base, "timeout")), "timeout"},
		{"kind wrapped by Info", orerr.Info(orerr.Kind(base, "timeout"), log.F{"a": 1}), "timeout"},
		{"kind wrapped by Retryable", orerr.Retryable(orerr.Kind(base, "timeout")), "timeout"},
		{"nearest wins", orerr.Kind(fmt.Errorf("x: %w", orerr.Kind(base, "inner")), "outer"), "outer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := events.NewErrorInfo(tt.err)
			assert.Equal(t, info.Kind, tt.want)
			assert.DeepEqual(t, kindValues(marshalEntries(info)), []interface{}{tt.want})
		})
	}
}

func TestErrorInfoKindKeepsLayout(t *testing.T) {
	// A kind wrapper must not change message/cause layout of the error.
	base := errors.New("base")
	plain := events.NewErrorInfo(base)
	kinded := events.NewErrorInfo(orerr.Kind(base, "k"))
	assert.Equal(t, kinded.Message, plain.Message)
	assert.Equal(t, kinded.Error, plain.Error)
	assert.Equal(t, len(kinded.Stack), len(plain.Stack))
	assert.Equal(t, kinded.Cause == nil, plain.Cause == nil)
}

func TestErrorInfoKindKeepsInfoFields(t *testing.T) {
	err := orerr.Kind(orerr.Info(errors.New("base"), log.F{"a": 1}), "k")
	got := map[string]interface{}{}
	events.NewErrorInfo(err).MarshalLog(func(k string, v interface{}) { got[k] = v })
	assert.Equal(t, got["a"], 1)
	assert.Equal(t, got["error.kind"], "k")
}

func TestErrorInfoKindNested(t *testing.T) {
	// Causes keep their "cause" kind; only the top-level error.kind changes.
	err := orerr.Kind(fmt.Errorf("outer: %w", errors.New("inner")), "k")
	info := events.NewErrorInfo(err)
	assert.Assert(t, info.Cause != nil)
	got := map[string]interface{}{}
	info.Cause.MarshalLog(func(k string, v interface{}) { got[k] = v })
	assert.Equal(t, got["kind"], "cause")
}

// A Custom marshaler's error.kind is dropped when an explicit kind is set, so
// the explicit kind deterministically wins.
func TestErrorInfoKindOverridesCustomMarshaler(t *testing.T) {
	// Via Info: the legacy mechanism emitting error.kind through Custom.
	viaInfo := orerr.Kind(orerr.Info(errors.New("base"), log.F{"error.kind": "legacy"}), "explicit")
	assert.DeepEqual(t, kindValues(marshalEntries(events.NewErrorInfo(viaInfo))), []interface{}{"explicit"})

	// Via a marshaler error type.
	viaMarshaler := orerr.Kind(kindMarshalerError{}, "explicit")
	entries := marshalEntries(events.NewErrorInfo(viaMarshaler))
	assert.DeepEqual(t, kindValues(entries), []interface{}{"explicit"})
	found := false
	for _, e := range entries {
		if e[0] == "extra" {
			found = true
		}
	}
	assert.Assert(t, found, "other custom fields must still be emitted")

	// Without an explicit kind the previous behavior is unchanged (both
	// entries emitted).
	legacy := orerr.Info(errors.New("base"), log.F{"error.kind": "legacy"})
	assert.DeepEqual(t, kindValues(marshalEntries(events.NewErrorInfo(legacy))), []interface{}{"error", "legacy"})
}
