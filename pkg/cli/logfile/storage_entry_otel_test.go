// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file implements tests for the trace serialization logic.

package logfile

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opentelemetry.io/otel/attribute"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gotest.tools/v3/assert"
)

func TestTraceRoundtripJSON(t *testing.T) {
	testFilePath := "testdata/trace.json"
	originalJSON, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("unable to read %s: %v", testFilePath, err)
	}

	var spans []Span
	if err := json.NewDecoder(bytes.NewReader(originalJSON)).Decode(&spans); err != nil {
		t.Fatalf("unable to decode %s: %v", testFilePath, err)
	}

	readOnlySpans := make([]tracesdk.ReadOnlySpan, len(spans))
	for i := range spans {
		readOnlySpans[i] = spans[i].Snapshot()
	}

	newJSON, err := json.MarshalIndent(tracetest.SpanStubsFromReadOnlySpans(readOnlySpans), "", "  ")
	assert.NilError(t, err, "failed to json marshal stub spans")

	if diff := cmp.Diff(strings.TrimSpace(string(originalJSON)), string(newJSON)); diff != "" {
		t.Fatalf("TestTraceRoundtripJSON() = %s", diff)
	}
}

func TestAsAttributeKeyValue(t *testing.T) {
	type args struct {
		Type  string
		value any
	}

	tests := []struct {
		name string
		args args
		want attribute.KeyValue
	}{
		{
			name: "string",
			args: args{
				Type:  attribute.STRING.String(),
				value: "value",
			},
			want: attribute.String("key", "value"),
		},
		{
			name: "int64 (int64)",
			args: args{
				Type:  attribute.INT64.String(),
				value: int64(1),
			},
			want: attribute.Int64("key", 1),
		},
		{
			name: "int64 (float64)",
			args: args{
				Type:  attribute.INT64.String(),
				value: float64(1.0),
			},
			want: attribute.Int64("key", 1),
		},
		{
			name: "bool",
			args: args{
				Type:  attribute.BOOL.String(),
				value: true,
			},
			want: attribute.Bool("key", true),
		},
		{
			name: "float64",
			args: args{
				Type:  attribute.FLOAT64.String(),
				value: float64(1.0),
			},
			want: attribute.Float64("key", 1.0),
		},
		{
			name: "float64slice",
			args: args{
				Type:  attribute.FLOAT64SLICE.String(),
				value: []float64{1.0, 2.0},
			},
			want: attribute.Float64Slice("key", []float64{1.0, 2.0}),
		},
		{
			name: "int64slice (int64)",
			args: args{
				Type:  attribute.INT64SLICE.String(),
				value: []int64{1, 2},
			},
			want: attribute.Int64Slice("key", []int64{1, 2}),
		},
		{
			name: "int64slice (float64)",
			args: args{
				Type:  attribute.INT64SLICE.String(),
				value: []float64{1.0, 2.0},
			},
			want: attribute.Int64Slice("key", []int64{1, 2}),
		},
		{
			name: "boolslice",
			args: args{
				Type:  attribute.BOOLSLICE.String(),
				value: []bool{true, false},
			},
			want: attribute.BoolSlice("key", []bool{true, false}),
		},
		{
			name: "stringslice (strings)",
			args: args{
				Type:  attribute.STRINGSLICE.String(),
				value: []string{"value1", "value2"},
			},
			want: attribute.StringSlice("key", []string{"value1", "value2"}),
		},
		{
			name: "stringslice (interface of string)",
			args: args{
				Type:  attribute.STRINGSLICE.String(),
				value: []interface{}{"value1", "value2"},
			},
			want: attribute.StringSlice("key", []string{"value1", "value2"}),
		},
		{
			name: "stringslice (interface mixed)",
			args: args{
				Type:  attribute.STRINGSLICE.String(),
				value: []interface{}{"value1", 2},
			},
			want: attribute.StringSlice("key", []string{"value1", "2"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := keyValue{
				Key:   "key",
				Value: value{Type: tt.args.Type, Value: tt.args.value},
			}

			attr, err := kv.asAttributeKeyValue()
			assert.NilError(t, err, "failed to convert key value to attribute key value")
			assert.DeepEqual(t, attr, tt.want, cmpopts.IgnoreUnexported(attribute.Value{}))
		})
	}
}
