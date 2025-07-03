package trace_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/codes"
	"gotest.tools/v3/assert"
)

type MarshalFunc func(addField func(key string, v interface{}))

func (mf MarshalFunc) MarshalLog(addField func(key string, v interface{})) {
	mf(addField)
}

type marshalableError struct {
	Err error
}

func (m *marshalableError) MarshalLog(addField func(key string, v interface{})) {
	if m == nil {
		return
	}
	addField("err", m.Err.Error())
}

func (m *marshalableError) Error() string {
	if m.Err != nil {
		return m.Err.Error()
	}
	return ""
}

func TestEvent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	defer sr.Close()
	ctx := context.Background()

	ctx = trace.StartSpan(ctx, "testspan")
	trace.SendEvent(ctx, "event", log.F{
		"hi": "friends",
	})
	trace.End(ctx)

	ended := sr.Recorder.Ended()
	if len(ended) > 1 {
		t.Fatal("expected a single span; got", len(ended))
	}

	for _, item := range ended {
		evs := item.Events()
		assert.Equal(t, len(evs), 1)
		for i := range evs {
			assert.Equal(t, evs[i].Name, "event")
			assert.Equal(t, string(evs[i].Attributes[0].Key), "hi")
			assert.Equal(t, evs[i].Attributes[0].Value.AsString(), "friends")
		}
	}
}

func TestTraceError(t *testing.T) {
	err := fmt.Errorf("test error")
	var customError = &marshalableError{
		Err: fmt.Errorf("party"),
	}

	type testArgs struct {
		input    error
		expected any
	}

	orErr := orerr.New(fmt.Errorf("oh no"), orerr.WithInfo(log.F{"details": "juice"}))
	cases := map[string]testArgs{
		"custom":     {customError, customError},
		"fmt.Errorf": {err, err},
		"orerr":      {orErr, orErr},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			sr := tracetest.NewSpanRecorder()
			defer sr.Close()
			ctx := trace.StartSpan(context.Background(), "test")
			err := trace.Error(ctx, v.input)
			assert.DeepEqual(t, err.Error(), v.input.Error())
			trace.End(ctx)

			ended := sr.Recorder.Ended()
			if len(ended) > 1 {
				t.Fatal("expected a single span; got", len(ended))
			}

			for _, item := range ended {
				assert.Equal(t, item.Status().Code, codes.Error)
				assert.Equal(t, item.Status().Description, v.input.Error())
				for _, ev := range item.Events() {
					assert.Equal(t, ev.Name, "exception")
					attrs := map[string]string{}
					for _, a := range ev.Attributes {
						attrs[string(a.Key)] = a.Value.Emit()
					}
					assert.Check(t, attrs["exception.message"] != "")
					assert.Check(t, attrs["exception.type"] != "")
				}
			}
		})
	}

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
