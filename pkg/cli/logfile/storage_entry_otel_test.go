// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file implements tests for the trace serialization logic.

package logfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	// TODO(jaredallard): When I land, lookup how to minify JSON w/o parsing it. If we parse it
	// the map key order gets messed up....
	originalJSON, err := exec.Command("sh", "-c", fmt.Sprintf("cat '%s' | jq -c . | tr -d '\\n'", testFilePath)).Output()
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

	if diff := cmp.Diff(string(originalJSON), string(newJSON)); diff != "" {
		t.Fatalf("TestTraceRoundtripJSON() = %s", diff)
	}
}
