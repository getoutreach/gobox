package telefork

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestClientDoesNotSendNoEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("No request should be made")
	}))
	defer server.Close()

	os.Setenv("OUTREACH_TELEFORK_ENDPOINT", server.URL)
	client := NewClientWithHTTPClient("testApp", "testKey", server.Client())

	client.Close()
}

func TestClientSilentlyFails(_ *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	os.Setenv("OUTREACH_TELEFORK_ENDPOINT", server.URL)
	client := NewClientWithHTTPClient("testApp", "testKey", server.Client())

	client.SendEvent([]attribute.KeyValue{attribute.String("key1", "val1")})

	client.Close()
}

func TestClientDoesNotSendReqWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("No request should be made")
	}))
	defer server.Close()

	os.Setenv("OUTREACH_TELEFORK_ENDPOINT", server.URL)
	client := NewClientWithHTTPClient("testApp", "NOTSET", server.Client())

	client.Close()
}

func TestClientSendsEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			t.Error("expected body")
		}
		defer r.Body.Close()

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		expectedVal := `[{"key1":"val1"}]`
		if string(b) != expectedVal {
			t.Logf("expected '%s', got '%s'", expectedVal, string(b))
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	os.Setenv("OUTREACH_TELEFORK_ENDPOINT", server.URL)
	client := NewClientWithHTTPClient("testApp", "testAPIKey", server.Client())

	client.SendEvent([]attribute.KeyValue{attribute.String("key1", "val1")})

	client.Close()
}

func TestClientCombinesDefaultInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			t.Error("expected body")
		}
		defer r.Body.Close()

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		expectedVal := `[{"key1":"val1","key2":"val2","req":"req1"},{"key1":"defaultVal1","key2":"val2","req":"req2"}]`
		if string(b) != expectedVal {
			t.Logf("expected '%s', got '%s'", expectedVal, string(b))
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	os.Setenv("OUTREACH_TELEFORK_ENDPOINT", server.URL)
	client := NewClientWithHTTPClient("test", "test", server.Client())

	client.AddField("key1", "defaultVal1")
	client.AddField("key2", "val2")
	client.SendEvent([]attribute.KeyValue{attribute.String("req", "req1"), attribute.String("key1", "val1")})
	client.SendEvent([]attribute.KeyValue{attribute.String("req", "req2")})

	client.Close()
}
