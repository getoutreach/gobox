// logtest provides the ability to test logs
//
// Usage:
//
//     func MyTestFunc(t *testing.T) {
//         logs := logTest.NewLogRecorder(t)
//         defer logs.Close()
//         .....
//         if diff := logs.Diff(expected); diff != "" {
//             t.Fatal("logs unexpected", diff);
//         }
//     }
package logtest

import (
	"encoding/json"
	"io"
	"sync"
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
)

// NewLogRecorder starts a new log recorder.
//
// Logs must be stopped by calling Close() on the recorder
func NewLogRecorder(t *testing.T) *LogRecorder {
	r := &LogRecorder{T: t, oldOutput: log.Output()}
	log.SetOutput(r)
	return r
}

// LogRecorder holds the state
type LogRecorder struct {
	*testing.T
	oldOutput io.Writer
	entries   []log.F
	sync.Mutex
}

func (l *LogRecorder) Write(b []byte) (n int, err error) {
	var entry log.F
	if err := json.Unmarshal(b, &entry); err != nil {
		l.Fatal("invalid log entry", err)
	}

	l.Lock()
	defer l.Unlock()
	l.entries = append(l.entries, entry)
	return
}

// Close closes the recorder
func (l *LogRecorder) Close() {
	log.SetOutput(l.oldOutput)
}

// Entries returns the log entries.
func (l *LogRecorder) Entries() []log.F {
	l.Lock()
	defer l.Unlock()
	return l.entries[:len(l.entries):len(l.entries)]
}

// MarshalToMap uses the given arguments `MarshalLog` function to serialize it
// into a map, which it returns.
//
// The serialization is similar to the one performed by logs.  Any nesting is
// flattened by representing it as dot-separated key prefixes in a flat map.
func Map(m log.Marshaler) map[string]interface{} {
	ret := log.F{}
	m.MarshalLog(ret.Set)
	return ret
}
