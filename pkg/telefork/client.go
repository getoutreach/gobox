package telefork

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/getoutreach/gobox/pkg/log"
	"go.opentelemetry.io/otel/attribute"
)

type Event map[string]interface{}

type Client interface {
	// Enqueues an event with the given attributes for later sending.
	SendEvent(attributes []attribute.KeyValue)

	// Add a common property that will be sent with all traces. These will be
	// overridden by event attributes with the same name (they are treated as
	// default values).
	AddField(key string, val interface{})
	AddInfo(args ...log.Marshaler)

	// Send all events that have been enqueued and close the client. The client
	// should be discarded after calling this method.
	Close()
}

func NewClient(appName, apiKey string) Client {
	c := &http.Client{}
	return NewClientWithHTTPClient(appName, apiKey, c)
}

func NewClientWithHTTPClient(appName, apiKey string, httpClient *http.Client) Client {
	baseURL := "https://telefork.outreach.io/"
	if os.Getenv("OUTREACH_TELEFORK_ENDPOINT") != "" {
		baseURL = os.Getenv("OUTREACH_TELEFORK_ENDPOINT")
	}
	return &client{
		http: httpClient,

		appName: appName,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

type client struct {
	http *http.Client

	appName string
	apiKey  string
	baseURL string
	events  []Event

	// Properties that are sent with every trace.
	commonProps map[string]interface{}
}

func (c *client) SendEvent(attributes []attribute.KeyValue) {
	e := make(Event)

	for k, v := range c.commonProps {
		e[k] = v
	}
	for _, a := range attributes {
		e[string(a.Key)] = a.Value.AsString()
	}

	c.events = append(c.events, e)
}

func (c *client) Close() {
	if c.apiKey == "" || c.apiKey == "NOTSET" {
		return
	}

	if len(c.events) == 0 {
		return
	}

	b, err := json.Marshal(c.events)
	if err != nil {
		return
	}

	r, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(c.baseURL, "/")+"/", bytes.NewReader(b))
	if err != nil {
		return
	}

	r.Header.Set("content-type", "application/json")
	r.Header.Set("x-outreach-client-logging", c.apiKey)
	r.Header.Set("x-outreach-client-app-id", c.appName)

	res, err := c.http.Do(r)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		// TODO: Retry? If necessary.
		return
	}
}

func (c *client) AddInfo(args ...log.Marshaler) {
	for _, arg := range args {
		arg.MarshalLog(c.AddField)
	}
}

func (c *client) AddField(key string, val interface{}) {
	if c.commonProps == nil {
		c.commonProps = map[string]interface{}{}
	}
	c.commonProps[key] = val
}
