package telefork

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/getoutreach/gobox/pkg/log"
)

type Event map[string]interface{}

type Client interface {
	SendEvent(event Event)

	AddField(key string, val interface{})
	AddInfo(args ...log.Marshaler)

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

	commonProps map[string]interface{}
}

func (c *client) SendEvent(event Event) {
	e := make(Event)

	for k, v := range c.commonProps {
		e[k] = v
	}
	for k, v := range event {
		e[k] = v
	}

	// We don't want certain fields added by honeycomb automatically.
	// We do add some of them under different keys we've used before in CLIs.
	// We skip adding them for non-@outreach.io emails. (Since it can be used in OSS projects)
	delete(e, "meta.beeline_version")
	delete(e, "meta.local_hostname")
	delete(e, "meta.span_type")

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
		fmt.Printf("failed to marshal events: %s\n", err)
		return
	}

	r, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(c.baseURL, "/")+"/", bytes.NewReader(b))
	if err != nil {
		fmt.Println(err)
		return
	}

	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-OUTREACH-CLIENT-LOGGING", c.apiKey)
	r.Header.Set("X-OUTREACH-CLIENT-APP-ID", c.appName)

	res, err := c.http.Do(r)
	if err != nil {
		fmt.Println(err)
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
