//go:build !or_e2e

package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"gotest.tools/v3/assert"
)

func TestUnmarshalableValues(t *testing.T) {
	t.Run("Infinity", func(t *testing.T) {
		var b bytes.Buffer
		log.SetOutput(&b)
		log.Info(context.Background(), "infinity is not fine but not a problem", log.F{"party": math.Inf(1)})
		assert.Assert(t, b.Len() > 0, "log should not be empty")
		f := log.F{}
		err := json.Unmarshal(b.Bytes(), &f)
		assert.NilError(t, err)
		_, ok := f["party"]
		assert.Assert(t, !ok, "party should not be present")
	})

	t.Run("Func", func(t *testing.T) {
		log.Info(context.Background(), "infinity is not fine but not a problem", log.F{"party": func() {}})
	})
}

func Example() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Info(context.Background(), "example", log.F{"myField": 42})
	log.Warn(context.Background(), "example", log.F{"myField": 42})
	log.Error(context.Background(), "example", log.F{"myField": 42})

	printEntries(logs.Entries())

	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"INFO","message":"example","module":"github.com/getoutreach/gobox","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"WARN","message":"example","module":"github.com/getoutreach/gobox","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"ERROR","message":"example","module":"github.com/getoutreach/gobox","myField":42}
}

func Example_with_custom_event() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Info(context.Background(), "example", MyEvent{"boo"})

	printEntries(logs.Entries())

	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"INFO","message":"example","module":"github.com/getoutreach/gobox","myevent_field":"boo"}
}

func ExampleDebug_with_error() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Debug(context.Background(), "msg1", log.F{"myField": 42})
	log.Error(context.Background(), "msg2", log.F{"myField": 42})

	printEntries(logs.Entries())
	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"DEBUG","message":"msg1","module":"github.com/getoutreach/gobox","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"ERROR","message":"msg2","module":"github.com/getoutreach/gobox","myField":42}
}

func Example_with_nested_custom_event() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Info(context.Background(), "example", log.F{
		"error": log.F{
			"cause": " error",
			"data":  MyEvent{"boo"},
		},
	})

	printEntries(logs.Entries())

	//nolint:lll // Why: Output
	// Output:
	//{"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","error.cause":" error","error.data.myevent_field":"boo","level":"INFO","message":"example","module":"github.com/getoutreach/gobox","rootfield":"value"}
}

func ExampleDebug_without_error() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Debug(context.Background(), "debug", log.F{"myField": 42})
	log.Info(context.Background(), "debug", log.F{"myField": 42})

	// trigger debug being pushed out but remove it from entries
	log.Error(context.Background(), "moo", nil)
	entries := logs.Entries()
	printEntries(entries[:len(entries)-1])

	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"INFO","message":"debug","module":"github.com/getoutreach/gobox","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"DEBUG","message":"debug","module":"github.com/getoutreach/gobox","myField":42}
}

func Example_appName() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	defer app.SetName(app.Info().Name)
	app.SetName("app_name")

	log.Info(context.Background(), "orgful", log.F{"myField": 42})

	printEntries(logs.Entries())
	//nolint:lll // Why: Output
	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.name":"app_name","app.version":"testing","level":"INFO","message":"orgful","module":"github.com/getoutreach/gobox","myField":42,"service_name":"app_name"}
}

func printEntries(entries []log.F) {
	for _, entry := range entries {
		entry["@timestamp"] = "2019-09-05T14:27:40Z"
		bytes, err := json.Marshal(entry)
		if err != nil {
			fmt.Println("unexpected", err)
		} else {
			fmt.Println(string(bytes))
		}
	}
}

// MyEvent demonstrates how custom events can be marshaled
type MyEvent struct {
	SomeField string
}

func (m MyEvent) MarshalRoot() log.Marshaler {
	return log.F{"rootfield": "value"}
}

func (m MyEvent) MarshalLog(addField func(field string, value interface{})) {
	addField("myevent_field", m.SomeField)
}
