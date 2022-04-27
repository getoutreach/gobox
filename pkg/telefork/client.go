package telefork

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

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
	endpoint := "https://telefork.outreach.io/"
	if os.Getenv("OUTREACH_TELEFORK_ENDPOINT") != "" {
		endpoint = os.Getenv("OUTREACH_TELEFORK_ENDPOINT")
	}
	return &client{
		appName:  appName,
		endpoint: endpoint,
		apiKey:   apiKey,
	}
}

type client struct {
	appName  string
	apiKey   string
	endpoint string
	events   []Event

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

	b, err := json.Marshal(c.events)
	if err != nil {
		fmt.Printf("failed to marshal events: %s\n", err)
		return
	}

	r, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(b))
	if err != nil {
		fmt.Println(err)
		return
	}

	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-OUTREACH-CLIENT-LOGGING", c.apiKey)
	r.Header.Set("X-OUTREACH-CLIENT-APP-ID", c.appName)

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		fmt.Println("status code:", res.StatusCode)
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
