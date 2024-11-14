//go:build !or_e2e

package events_test

import (
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/google/go-cmp/cmp"
)

type eventsSuite struct{}

func (eventsSuite) TestHTTPRequest(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost/myendpoint/1", http.NoBody)
	if err != nil {
		t.Fatal("Unexpected err", err)
	}

	req = req.WithContext(events.WithRequestRoute(req.Context(), "/myendpoint/:id"))

	req.Header.Add("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	xrs := time.Now().Add(-time.Minute)
	seconds := xrs.Unix()
	ms := (xrs.UnixNano() % 1000000) / 1000
	req.Header.Add("X-Request-Start", fmt.Sprintf("%d.%03d", seconds, ms))

	var info events.HTTPRequest
	info.FillFieldsFromRequest(req)
	info.FillResponseInfo(100, 202)

	fields := map[string]interface{}{}
	info.MarshalLog(addFields(fields, ""))

	expected := events.HTTPRequest{
		NetworkRequest: events.NetworkRequest{BytesWritten: 100, RemoteAddr: "1.1.1.1"},
		Times: events.Times{
			Scheduled: xrs,
			Started:   time.Now(),
			Finished:  time.Now(),
		},
		Durations: events.Durations{
			ServiceSeconds: 0,
			WaitSeconds:    60,
			TotalSeconds:   60,
		},
		Duration:   0,
		Method:     "GET",
		StatusCode: 202,
		Path:       "/myendpoint/1",
		Route:      "/myendpoint/:id",
	}
	if diff := cmp.Diff(info, expected, cmp.Comparer(approxTime), cmp.Comparer(approxFloat)); diff != "" {
		t.Fatal("unexpected", diff)
	}
}

func approxTime(x, y time.Time) bool {
	return math.Abs(float64(y.Sub(x))/float64(time.Second)) < 1
}

func approxFloat(x, y float64) bool {
	return math.Abs(y-x) < 1
}

// addFields returns a addField function that can be used with MarshalLog
//
// The provided map is where the results of marshal log would be stored.
func addFields(m map[string]interface{}, prefix string) func(key string, v interface{}) {
	return func(key string, v interface{}) {
		if prefix != "" {
			key = prefix + "." + key
		}
		if marshaler, ok := v.(log.Marshaler); ok {
			marshaler.MarshalLog(addFields(m, key))
		} else {
			m[key] = v
		}
	}
}
