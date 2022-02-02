package metrics_test

import (
	"fmt"
	"time"

	_ "github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func Example() {
	start := time.Now()
	// sleep for 6ms to cross the lower most bucket of 5ms
	time.Sleep(6 * time.Millisecond)
	latency := float64(time.Since(start)) / float64(time.Second)
	metrics.ReportHTTPLatency("example_app", "example_call", latency, nil)

	got, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for _, metricFamily := range got {
		if metricFamily.GetName() == "http_request_handled" {
			for _, metric := range metricFamily.Metric {
				found := false
				for _, labelPair := range metric.GetLabel() {
					if labelPair.GetName() == "app" && labelPair.GetValue() == "example_app" {
						found = true
					}
				}
				if !found {
					continue
				}
				fmt.Println("name", metricFamily.GetName())
				fmt.Println("help", metricFamily.GetHelp())
				fmt.Println("type", metricFamily.GetType())
				fmt.Println("label", metric.GetLabel())
				fmt.Println("summary", metric.GetSummary())
				fmt.Println("sample count", metric.GetHistogram().GetSampleCount())
				fmt.Println("sample count", metric.GetHistogram().GetBucket())
			}
		}
	}

	//nolint:lll // Why: Output comparision
	// Output:
	// name http_request_handled
	// help The latency of the HTTP request, in seconds
	// type HISTOGRAM
	// label [name:"app" value:"example_app"  name:"call" value:"example_call"  name:"kind" value:"internal"  name:"statuscategory" value:"CategoryOK"  name:"statuscode" value:"OK" ]
	// summary <nil>
	// sample count 1
	// sample count [cumulative_count:0 upper_bound:0.005  cumulative_count:1 upper_bound:0.01  cumulative_count:1 upper_bound:0.025  cumulative_count:1 upper_bound:0.05  cumulative_count:1 upper_bound:0.1  cumulative_count:1 upper_bound:0.25  cumulative_count:1 upper_bound:0.5  cumulative_count:1 upper_bound:1  cumulative_count:1 upper_bound:2.5  cumulative_count:1 upper_bound:5  cumulative_count:1 upper_bound:10 ]
}
