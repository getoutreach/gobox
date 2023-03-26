// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file implements tests for the trace serialization logic.

package logfile

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tdewolff/minify/v2"
	minifyjson "github.com/tdewolff/minify/v2/json" // used by minify
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gotest.tools/v3/assert"
)

func TestTraceRoundtripJSON(t *testing.T) {
	testFilePath := "testdata/trace.json"
	originalJSONStr, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("unable to read %s: %v", testFilePath, err)
	}

	var originalJSON bytes.Buffer
	err = minifyjson.Minify(minify.New(), &originalJSON, bytes.NewReader(originalJSONStr), nil)
	assert.NilError(t, err, "failed to minify original json")

	var spans []Span
	if err := json.NewDecoder(bytes.NewReader(originalJSONStr)).Decode(&spans); err != nil {
		t.Fatalf("unable to decode %s: %v", testFilePath, err)
	}

	readOnlySpans := make([]tracesdk.ReadOnlySpan, len(spans))
	for i := range spans {
		readOnlySpans[i] = spans[i].Snapshot()
	}

	newJSON, err := json.Marshal(tracetest.SpanStubsFromReadOnlySpans(readOnlySpans))
	assert.NilError(t, err, "failed to json marshal stub spans")

	if diff := cmp.Diff(originalJSON.String(), string(newJSON)); diff != "" {
		t.Fatalf("TestTraceRoundtripJSON() = %s", diff)
	}
}
