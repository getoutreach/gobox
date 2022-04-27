package cli

import (
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
