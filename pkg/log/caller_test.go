//go:build !or_e2e

package log_test

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
)

type callerSuite struct{}

func (callerSuite) TestCaller(t *testing.T) {
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()
	log.Info(ctx, "caller test", log.Caller())

	expected := []log.F{{
		"@timestamp":  differs.RFC3339NanoTime(),
		"app.version": "testing",
		"caller":      "gobox/pkg/log/caller_test.go:23",
		"level":       "INFO",
		"message":     "caller test",
		"module":      "github.com/getoutreach/gobox",
	}}

	assert.DeepEqual(t, expected, logs.Entries(), differs.Custom())
}
