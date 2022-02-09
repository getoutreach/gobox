package tracelog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/getoutreach/gobox/x/tracelog"
	"gotest.tools/v3/assert"
)

type RowID string

func (r RowID) MarshalLog(addField func(k string, v interface{})) {
	addField("sql.row.id", string(r))
}

type TableName string

func (t TableName) MarshalLog(addField func(k string, v interface{})) {
	addField("sql.table", string(t))
}

type Model struct {
	ID        RowID
	TableName TableName
}

func (m *Model) MarshalLog(addField func(k string, v interface{})) {
	addField("model", "my model")
}

func TestNestedCall(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLog()
	defer trlog.Close()
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()

	//  most functions should look like this
	doSomeTableUpdate := func(ctx context.Context, rowID RowID, tableName TableName) error {
		ctx = trace.StartCall(ctx, "sql", rowID, tableName)
		defer trace.EndCall(ctx)

		trace.AddInfo(ctx, log.F{"info_1_key": "info_1_val", "info_2_key": "info_2_val"})
		// do some query work

		// report errors
		return trace.SetCallStatus(ctx, errors.New("sql error"))
	}

	// *model* function calls doSomeTableUpdate
	outer := func(ctx context.Context, m *Model) error {
		ctx = trace.StartCall(ctx, "model", m)
		defer trace.EndCall(ctx)

		trace.AddInfo(ctx, log.F{"info_3_key": "info_3_val", "info_4_key": "info_4_val"})

		// Note that model is added to trace.Info so that nested calls can log it.
		trace.AddInfo(ctx, m)
		return trace.SetCallStatus(ctx, doSomeTableUpdate(ctx, m.ID, m.TableName))
	}

	// wrapping the main logic in a function so that we can call
	// defer per our accepted trace.StartTrace/trace.End pattern
	func() {
		ctx = trace.StartTrace(ctx, "trace-test")
		defer trace.End(ctx)

		trace.AddProvider(tracelog.New(tracelog.WithAllInheritedArgs()))
		if err := outer(ctx, &Model{ID: "some model id", TableName: "some table"}); err == nil {
			t.Fatal("unexpected success", err)
		}
	}()

	outerSpanID, outerParentID, innerSpanID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()
	expectedLogs := []log.F{
		{
			"honeycomb.parent_id": outerParentID,
			"honeycomb.span_id":   outerSpanID,
			"level":               "INFO",
			"message":             "calling: model",
			"model":               "my model",
		},
		{
			"honeycomb.parent_id": outerSpanID,
			"honeycomb.span_id":   innerSpanID,
			"info_3_key":          "info_3_val",
			"info_4_key":          "info_4_val",
			"level":               "INFO",
			"message":             "calling: sql",
			"model":               "my model",
			"sql.row.id":          "some model id",
			"sql.table":           "some table",
		},
		{
			"honeycomb.parent_id": outerSpanID,
			"honeycomb.span_id":   innerSpanID,
			"error.error":         "sql error",
			"error.kind":          "error",
			"error.message":       "sql error",
			"info_1_key":          "info_1_val",
			"info_2_key":          "info_2_val",
			"info_3_key":          "info_3_val",
			"info_4_key":          "info_4_val",
			"level":               "ERROR",
			"message":             "called: sql",
			"model":               "my model",
			"sql.row.id":          "some model id",
			"sql.table":           "some table",
		},
		{
			"honeycomb.parent_id": outerParentID,
			"honeycomb.span_id":   outerSpanID,
			"error.error":         "sql error",
			"error.kind":          "error",
			"error.message":       "sql error",
			"info_3_key":          "info_3_val",
			"info_4_key":          "info_4_val",
			"level":               "ERROR",
			"message":             "called: model",
			"model":               "my model",
		},
	}

	actualLogs := []log.F{}
	for _, entry := range logs.Entries() {
		if entry["level"] == "DEBUG" || entry["event_name"] == "trace" {
			// these were not generated via tracelog.  Ignore these
			continue
		}
		actualLogs = append(actualLogs, entry)
	}

	assert.Equal(t, len(expectedLogs), len(actualLogs))

	// add common fields
	for _, entry := range expectedLogs {
		entry["@timestamp"] = differs.RFC3339NanoTime()
		entry["app.name"] = "gobox"
		entry["app.version"] = "testing"
		entry["honeycomb.trace_id"] = differs.CaptureString()
		entry["timing.dequeued_at"] = differs.RFC3339NanoTime()
		entry["timing.finished_at"] = differs.RFC3339NanoTime()
		entry["timing.scheduled_at"] = differs.RFC3339NanoTime()
		entry["timing.service_time"] = differs.FloatRange(0, 5)
		entry["timing.total_time"] = differs.FloatRange(0, 5)
		entry["timing.wait_time"] = differs.FloatRange(0, 5)
	}

	assert.DeepEqual(t, expectedLogs, actualLogs, differs.Custom())
}
