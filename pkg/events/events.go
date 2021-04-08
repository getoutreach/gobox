// Package events defines the standard logging event structures
//
// This is based on
// https://docs.google.com/document/d/1V1py1iXX9B9NAb30veHYNGOuymZ9o_C2pYSU9E6qmsg/edit#
// and
// https://outreach-io.atlassian.net/wiki/spaces/EN/pages/691405109/Logging+Standards
package events

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// User is how we log user info
type User struct {
	ID    string `log:"usr.id"`
	Email string `log:"usr.email"`
}

// Generate all the marshalers for types here by running go generate
//go:generate go run github.com/getoutreach/gobox/tools/logger

// Org represents the details for an org.
//
// See ExampleEmbeddedOrgEvent for embedded Org within other events
type Org struct {
	Bento         string `log:"or.org.bento"`
	DatabaseHost  string `log:"or.org.database_host"`
	ShortName     string `log:"or.org.shortname"`
	GUID          string `log:"or.org.guid"`
	ReportingTeam string `log:"or.org.reporting_team"`
}

// ExampleEmbeddedOrgEvent is mostly an example for how to embed
// events that are already fully namespaced (such as Org)
type ExampleEmbeddedOrgEvent struct {
	Org `log:"."` // the dot tells tools/logger to marshal org as is
}

// Durations holds the various times in seconds
type Durations struct {
	ServiceSeconds float64 `log:"timing.service_time"`
	WaitSeconds    float64 `log:"timing.wait_time"`
	TotalSeconds   float64 `log:"timing.total_time"`
}

// Times holds the actual times and also provides a convenient way to
// calculate the associated durations
type Times struct {
	Scheduled time.Time `log:"timing.scheduled_at"`
	Started   time.Time `log:"timing.dequeued_at"`
	Finished  time.Time `log:"timing.finished_at"`
}

// Durations returns the durations associated with the times
func (t *Times) Durations() *Durations {
	scheduled := t.Scheduled
	if t.Scheduled.IsZero() || t.Scheduled.After(t.Started) {
		scheduled = t.Started
	}

	svcDiff := t.Finished.Sub(t.Started)
	waitDiff := t.Started.Sub(scheduled)
	totalDiff := t.Finished.Sub(scheduled)

	return &Durations{
		ServiceSeconds: float64(svcDiff) / float64(time.Second),
		WaitSeconds:    float64(waitDiff) / float64(time.Second),
		TotalSeconds:   float64(totalDiff) / float64(time.Second),
	}
}

// NetworkRequest tracks network request related information
type NetworkRequest struct {
	BytesRead    int    `log:"network.bytes_read"`
	BytesWritten int    `log:"network.bytes_written"`
	RemoteAddr   string `log:"network.client.ip"`
	DestAddr     string `log:"network.destination.ip"`
}

// HTTPRequest tracks HTTP request related information
type HTTPRequest struct {
	// embed the network requests
	NetworkRequest `log:"."`

	// embed times
	Times `log:"."`

	// embed timing
	Durations `log:"."`

	// Duration is same as Durations.ServiceTime
	Duration float64 `log:"duration"`

	Method     string `log:"http.method"`
	Referer    string `log:"http.referer"`
	RequestID  string `log:"http.request_id"`
	StatusCode int    `log:"http.status_code"`
	Path       string `log:"http.url_details.path"`
	URI        string `log:"http.url_details.uri"`
	Endpoint   string `log:"http.url_details.endpoint"`
}

// FillFieldsFromRequest fills in the standard request fields
//
// Call FillResponseInfo() before logging this.
func (h *HTTPRequest) FillFieldsFromRequest(r *http.Request) {
	h.Method = r.Method
	h.Path = r.URL.Path
	h.URI = r.RequestURI
	h.Referer = r.Referer()
	h.RequestID = r.Header.Get("X-Request-ID")
	h.BytesRead, _ = strconv.Atoi(r.Header.Get("Content-Length")) //nolint: errcheck
	h.RemoteAddr = h.getRemoteAddr(r)
	h.Times.Scheduled = h.getXRequestStart(r)
	h.Times.Started = time.Now()
}

// FillResponseInfo fills in the response data as well as ends
// the timing information.
//
// Only the first call of this function has any effect.  After the
// Finished time has been updated, further calls will be ignored
func (h *HTTPRequest) FillResponseInfo(bytesWritten, statusCode int) {
	if h.Finished.IsZero() {
		h.Finished = time.Now()
		h.BytesWritten = bytesWritten
		h.StatusCode = statusCode
		h.Durations = *h.Times.Durations()
		h.Duration = h.Durations.ServiceSeconds
	}
}

func (h *HTTPRequest) getXRequestStart(r *http.Request) time.Time {
	ts, err := strconv.ParseFloat(r.Header.Get("X-Request-Start"), 64)
	if err != nil {
		return time.Now()
	}
	ns := int64(ts*1000*1000*1000) % 1000000000
	return time.Unix(int64(ts), ns)
}

func (h *HTTPRequest) getRemoteAddr(r *http.Request) string {
	// See: ETC-182.  Use a remote address library
	return strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]
}
