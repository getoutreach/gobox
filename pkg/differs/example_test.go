package differs_test

import (
	"fmt"
	"time"

	"github.com/getoutreach/gobox/pkg/differs"
	_ "github.com/getoutreach/gobox/pkg/log"
	"github.com/google/go-cmp/cmp"
)

func Example() {
	// RFC3339Time
	actual := map[string]interface{}{
		"rfc3339":       time.Now().Format(time.RFC3339),
		"any string":    "some string",
		"capture":       "captured value",
		"check capture": "captured value",
		"stack":         "some\nlong\ntack\ntrace",
		"float":         4.5,
	}
	capture := differs.CaptureString()
	expected := map[string]interface{}{
		"rfc3339":       differs.RFC3339Time(),
		"any string":    differs.AnyString(),
		"capture":       capture,
		"check capture": capture,
		"stack":         differs.StackLike("some\nlong\nstack"),
		"float":         differs.FloatRange(4, 5),
	}
	diff := cmp.Diff(expected, actual, differs.Custom())
	fmt.Println(diff)

	// RFC3339NanoTime
	actual = map[string]interface{}{
		"rfc3339nano":   time.Now().Format(time.RFC3339Nano),
		"any string":    "some string",
		"capture":       "captured value",
		"check capture": "captured value",
		"stack":         "some\nlong\ntack\ntrace",
		"float":         4.5,
	}
	capture = differs.CaptureString()
	expected = map[string]interface{}{
		"rfc3339nano":   differs.RFC3339NanoTime(),
		"any string":    differs.AnyString(),
		"capture":       capture,
		"check capture": capture,
		"stack":         differs.StackLike("some\nlong\nstack"),
		"float":         differs.FloatRange(4, 5),
	}
	diff = cmp.Diff(expected, actual, differs.Custom())
	fmt.Println(diff)

	// Output:
}
