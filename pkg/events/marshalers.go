// Code generated by "logger "; DO NOT EDIT.

package events

import "time"

func (s *Durations) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

	addField("timing.service_time", s.ServiceSeconds)
	addField("timing.wait_time", s.WaitSeconds)
	addField("timing.total_time", s.TotalSeconds)
}

func (s *HTTPRequest) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

	s.NetworkRequest.MarshalLog(addField)
	s.Times.MarshalLog(addField)
	s.Durations.MarshalLog(addField)
	addField("duration", s.Duration)
	addField("http.method", s.Method)
	addField("http.referer", s.Referer)
	addField("http.request_id", s.RequestID)
	addField("http.status_code", s.StatusCode)
	addField("http.url_details.path", s.Path)
	addField("http.url_details.uri", s.URI)
	addField("http.url_details.endpoint", s.Endpoint)
}

func (s *NetworkRequest) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

	addField("network.bytes_read", s.BytesRead)
	addField("network.bytes_written", s.BytesWritten)
	addField("network.client.ip", s.RemoteAddr)
	addField("network.destination.ip", s.DestAddr)
}

func (s *Times) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

	addField("timing.scheduled_at", s.Scheduled.UTC().Format(time.RFC3339Nano))
	addField("timing.dequeued_at", s.Started.UTC().Format(time.RFC3339Nano))
	addField("timing.finished_at", s.Finished.UTC().Format(time.RFC3339Nano))
}