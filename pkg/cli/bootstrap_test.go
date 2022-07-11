package cli

import (
	"context"
	"runtime"
	"testing"
)

func TestCommonProps(t *testing.T) {
	lm := commonProps()

	props := make(map[string]interface{})
	lm.MarshalLog(func(key string, v interface{}) {
		props[key] = v
	})

	if props["os.name"] != runtime.GOOS {
		t.Errorf("expected '%s', got '%s'", runtime.GOOS, props["os.name"])
	}
	if props["os.arch"] != runtime.GOARCH {
		t.Errorf("expected '%s', got '%s'", runtime.GOARCH, props["os.arch"])
	}
}

func TestSetupTracer(t *testing.T) {
	t.Log(`Verify that we don't panic when calling setupTracer.

This covers a regression where we didn't provide enough OpenTelemetry setup
in overrideConfigLoaders which caused setupTracer to panic.

Typically we should try to test the public interfaces (i.e. HookInUrfaveCLI),
but that causes the actual CLI to be executed (it ends up calling app.RunContext),
which is trickier to test.

Hence, we are calling private functions in the test, 
which are more prone to change over time. 
Since it's a simple test, the tradeoff seems reasonable.`)
	overrideConfigLoaders("", "", false)
	ctx := context.Background()
	setupTracer(ctx, t.Name())
}
