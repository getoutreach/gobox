package log_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
)

func Example() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Info(context.Background(), "example", log.F{"myField": 42})
	log.Warn(context.Background(), "example", log.F{"myField": 42})
	log.Error(context.Background(), "example", log.F{"myField": 42})

	printEntries(logs.Entries())

	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"INFO","message":"example","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"WARN","message":"example","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"ERROR","message":"example","myField":42}
}

func Example_with_custom_event() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Info(context.Background(), "example", MyEvent{"boo"})

	printEntries(logs.Entries())

	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"INFO","message":"example","myevent_field":"boo"}
}

func ExampleDebug_with_error() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	log.Debug(context.Background(), "msg1", log.F{"myField": 42})
	log.Error(context.Background(), "msg2", log.F{"myField": 42})

	printEntries(logs.Entries())
	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"DEBUG","message":"msg1","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"ERROR","message":"msg2","myField":42}
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
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"INFO","message":"debug","myField":42}
	// {"@timestamp":"2019-09-05T14:27:40Z","app.version":"testing","level":"DEBUG","message":"debug","myField":42}
}

func Example_appName() {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	defer app.SetName(app.Info().Name)
	app.SetName("app_name")

	log.Info(context.Background(), "orgful", log.F{"myField": 42})

	printEntries(logs.Entries())
	// Output:
	// {"@timestamp":"2019-09-05T14:27:40Z","app.name":"app_name","app.version":"testing","level":"INFO","message":"orgful","myField":42}
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

func (m MyEvent) MarshalLog(addField func(field string, value interface{})) {
	addField("myevent_field", m.SomeField)
}
