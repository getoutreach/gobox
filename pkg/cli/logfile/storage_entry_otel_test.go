// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file implements tests for the trace serialization logic.

package logfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceRoundtripJSON(t *testing.T) {
	t.Skip("Broken due to formatter")

	testFilePath := "testdata/trace.json"
	originalJSON, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("unable to read %s: %v", testFilePath, err)
	}

	var spans []Span
	if err := json.NewDecoder(bytes.NewReader(originalJSON)).Decode(&spans); err != nil {
		t.Fatalf("unable to decode %s: %v", testFilePath, err)
	}

	fmt.Printf("TraceID: %s\n", spans[0].SpanContext.TraceID())

	var readOnlySpans []tracesdk.ReadOnlySpan
	for _, span := range spans {
		readOnlySpans = append(readOnlySpans, span.Snapshot())
	}

	stubs := tracetest.SpanStubsFromReadOnlySpans(readOnlySpans)

	var newJSON bytes.Buffer
	if err := json.NewEncoder(&newJSON).Encode(stubs); err != nil {
		t.Fatalf("unable to encode %s: %v", testFilePath, err)
	}

	if !bytes.Equal(originalJSON, newJSON.Bytes()) {
		t.Fatalf("expectd: %s, to equal: %s", originalJSON, newJSON.String())
	}
}
