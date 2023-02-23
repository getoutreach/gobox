//go:build !or_e2e

package trace_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
)

type MarshalFunc func(addField func(key string, v interface{}))

func (mf MarshalFunc) MarshalLog(addField func(key string, v interface{})) {
	mf(addField)
}

func TestOtelAddInfo(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	defer sr.Close()

	ctx := context.Background()

	// OTel only understands a limited set of types (bool, string int64,
	// float64, and slices of these), so some casting is expected.
	//
	// There's also a handful of special cases where we provide special
	// handling, as in the case of `time.Time`.
	cases := map[string]struct{ value, expected interface{} }{
		// Exhaustive test of the bools.
		"false": {false, false},
		"true":  {true, true},

		// Signed integers: cast to int64.
		"int":   {int(-42), int64(-42)},
		"int8":  {int8(math.MaxInt8), int64(math.MaxInt8)},
		"int16": {int16(math.MaxInt16), int64(math.MaxInt16)},
		"int32": {int32(math.MaxInt32), int64(math.MaxInt32)},
		"int64": {int64(math.MaxInt64), int64(math.MaxInt64)},

		// Small unsigned integers: cast to int64.
		"uint8":  {uint8(math.MaxUint8), int64(math.MaxUint8)},
		"uint16": {uint16(math.MaxUint16), int64(math.MaxUint16)},
		"uint32": {uint32(math.MaxUint32), int64(math.MaxUint32)},

		// Sadly, these might not fit in an int64.  Cast to string.
		"uint":   {uint(42), "42"},
		"uint64": {uint64(math.MaxUint64), fmt.Sprintf("%d", uint64(math.MaxUint64))},

		// Floats: cast to float64.
		"float32": {float32(3.14), differs.FloatRange(3.10, 3.20)},
		"float64": {float64(2.718), differs.FloatRange(2.71, 2.72)},

		// String: simple enough.
		"string": {"hello world", "hello world"},

		// Slices of bools.
		"[]bool": {[]bool{false, true}, []bool{false, true}},

		// Slices of ints: cast to slices of int64.
		"[]int":   {[]int{-123, 123}, []int64{-123, 123}},
		"[]int8":  {[]int8{-123, 123}, []int64{-123, 123}},
		"[]int16": {[]int16{-123, 12300}, []int64{-123, 12300}},
		"[]int32": {[]int32{-123, 123000}, []int64{-123, 123000}},
		"[]int64": {[]int64{-123, 123000}, []int64{-123, 123000}},

		// Slices of small uints: cast to slices of int64.
		"[]uint8":  {[]uint8{111, 123}, []int64{111, 123}},
		"[]uint16": {[]uint16{111, 12300}, []int64{111, 12300}},
		"[]uint32": {[]uint32{111, 123000}, []int64{111, 123000}},

		// Slices of large uints: cast to string, unfortunately.
		"[]uint":   {[]uint{111, 123}, []string{"111", "123"}},
		"[]uint64": {[]uint64{111, 123000}, []string{"111", "123000"}},

		// Slices of strings.
		"[]string": {[]string{"hello", "world"}, []string{"hello", "world"}},

		// Slices of floats: cast to slices of float64.
		"[]float32": {[]float32{1.0, 2.0}, []float64{1.0, 2.0}},
		"[]float64": {[]float64{1.0, 2.0}, []float64{1.0, 2.0}},

		// Special handling for time.Time.
		"time.Time": {time.Unix(1668554262, 0), differs.RFC3339NanoTime()},
	}

	fn := func(addField func(key string, v interface{})) {
		for k, v := range cases {
			addField(k, v.value)
		}
	}

	expected := map[string]interface{}{
		"SampleRate":             int64(1),
		"startTime":              differs.AnyString(),
		"endTime":                differs.AnyString(),
		"name":                   "testspan",
		"attributes.app.version": "testing",
		"parent.remote":          bool(false),
		"parent.spanID":          differs.AnyString(),
		"parent.traceID":         differs.AnyString(),
		"parent.traceFlags":      differs.AnyString(),
		"spanContext.spanID":     differs.AnyString(),
		"spanContext.traceID":    differs.AnyString(),
		"spanContext.traceFlags": differs.AnyString(),
		"spanKind":               "internal",
	}
	for k, v := range cases {
		expected[fmt.Sprintf("attributes.%s", k)] = v.expected
	}

	ctx = trace.StartSpan(ctx, "testspan", MarshalFunc(fn))
	trace.End(ctx)

	ev := sr.Ended()

	if diff := cmp.Diff(expected, ev[0], differs.Custom()); diff != "" {
		t.Fatal("unexpected serialization", diff)
	}
}
