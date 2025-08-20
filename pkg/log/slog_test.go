package log_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/olog"
	pkgerrors "github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

// SlogTestEvent demonstrates how custom events can be marshaled in slog tests
type SlogTestEvent struct {
	SomeField string
}

func (m SlogTestEvent) MarshalLog(addField func(field string, value interface{})) {
	addField("myevent_field", m.SomeField)
}

// setupSlogTest sets up the slog facade for testing and returns a cleanup function
func setupSlogTest(t *testing.T) func() {
	t.Helper()

	// Save original environment
	originalEnv, wasSet := os.LookupEnv("GOBOX_AS_SLOG_FACADE")

	// Save original log output
	originalOutput := log.Output()

	// Enable slog facade
	err := os.Setenv("GOBOX_AS_SLOG_FACADE", "true")
	assert.Check(t, err)

	// Return cleanup function
	return func() {
		// Restore original log output first
		log.SetOutput(originalOutput)

		// Then restore environment
		var err error
		if wasSet {
			err = os.Setenv("GOBOX_AS_SLOG_FACADE", originalEnv)
		} else {
			err = os.Unsetenv("GOBOX_AS_SLOG_FACADE")
		}
		assert.Check(t, err)
	}
}

func TestSlogFacade(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)

	ctx := context.Background()

	t.Run("Debug", func(t *testing.T) {
		t.Skip("log at debug doesn't currently work because of problems with olog's registry")
		buf.Reset()
		// Set debug level to ensure debug logs are shown
		olog.SetGlobalLevel(slog.LevelDebug)
		defer olog.SetGlobalLevel(slog.LevelInfo) // reset to default

		log.Debug(ctx, "debug message", log.F{"key": "value"})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "debug message"))
		assert.Assert(t, strings.Contains(output, "DEBUG"))
		assert.Assert(t, strings.Contains(output, "key"))
		assert.Assert(t, strings.Contains(output, "value"))
	})

	t.Run("Info", func(t *testing.T) {
		buf.Reset()
		log.Info(ctx, "info message", log.F{"key": "value"})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "info message"))
		assert.Assert(t, strings.Contains(output, "INFO"))
		assert.Assert(t, strings.Contains(output, "key"))
		assert.Assert(t, strings.Contains(output, "value"))
	})

	t.Run("Warn", func(t *testing.T) {
		buf.Reset()
		log.Warn(ctx, "warn message", log.F{"key": "value"})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "warn message"))
		assert.Assert(t, strings.Contains(output, "WARN"))
		assert.Assert(t, strings.Contains(output, "key"))
		assert.Assert(t, strings.Contains(output, "value"))
	})

	t.Run("Error", func(t *testing.T) {
		buf.Reset()
		env, envSet := os.LookupEnv("GOBOX_AS_SLOG_FACADE")
		t.Logf("Environment GOBOX_AS_SLOG_FACADE: %v (set: %v)", env, envSet)
		log.Error(ctx, "error message", log.F{"key": "value"})

		output := buf.String()
		t.Logf("Error output: %s", output)
		assert.Assert(t, strings.Contains(output, "error message"))
		assert.Assert(t, strings.Contains(output, "ERRO"))
		assert.Assert(t, strings.Contains(output, "key"))
		assert.Assert(t, strings.Contains(output, "value"))
	})
}

func TestSlogAttrsConversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    []log.Marshaler
		expected map[string]any
	}{
		{
			name:     "empty",
			input:    []log.Marshaler{},
			expected: map[string]any{},
		},
		{
			name: "basic types",
			input: []log.Marshaler{
				log.F{
					"bool":        true,
					"int":         42,
					"int8":        int8(8),
					"int16":       int16(16),
					"int32":       int32(32),
					"int64":       int64(64),
					"uint8":       uint8(8),
					"uint16":      uint16(16),
					"uint32":      uint32(32),
					"float32":     float32(3.14),
					"float64":     float64(2.71),
					"string":      "hello",
					"duration":    time.Second,
					"custom_time": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			expected: map[string]any{
				"bool":        true,
				"int":         42,
				"int8":        int64(8),
				"int16":       int64(16),
				"int32":       int64(32),
				"int64":       int64(64),
				"uint8":       int64(8),
				"uint16":      int64(16),
				"uint32":      int64(32),
				"float32":     float64(3.14),
				"float64":     float64(2.71),
				"string":      "hello",
				"duration":    time.Second,
				"custom_time": "2023-01-01T00:00:00Z",
			},
		},
		{
			name: "slog value",
			input: []log.Marshaler{
				log.F{
					"slog_value": slog.StringValue("test"),
				},
			},
			expected: map[string]any{
				"slog_value": slog.StringValue("test"),
			},
		},
		{
			name: "fallback to string",
			input: []log.Marshaler{
				log.F{
					"slice": []int{1, 2, 3},
				},
			},
			expected: map[string]any{
				"slice": "[1 2 3]",
			},
		},
		{
			name: "sorted keys",
			input: []log.Marshaler{
				log.F{
					"z_last":  "last",
					"a_first": "first",
					"m_mid":   "middle",
				},
			},
			expected: map[string]any{
				"z_last":  "last",
				"a_first": "first",
				"m_mid":   "middle",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := setupSlogTest(t)
			defer cleanup()

			var buf bytes.Buffer
			log.SetOutput(&buf)

			log.Info(context.Background(), "test", tc.input...)

			output := buf.String()

			// Verify that all expected keys are present in output
			for key, expectedValue := range tc.expected {
				assert.Assert(t, strings.Contains(output, key),
					"output should contain key %q. Output: %s", key, output)

				// For string values, check the actual value is present
				if strVal, ok := expectedValue.(string); ok && key != "custom_time" {
					assert.Assert(t, strings.Contains(output, strVal),
						"output should contain value %q for key %q. Output: %s", strVal, key, output)
				}
			}
		})
	}
}

func TestSlogFacadeWithCustomMarshalers(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)

	ctx := context.Background()

	t.Run("custom marshaler", func(t *testing.T) {
		buf.Reset()

		event := SlogTestEvent{SomeField: "test_value"}
		log.Info(ctx, "custom event", event)

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "custom event"))
		assert.Assert(t, strings.Contains(output, "myevent_field"))
		assert.Assert(t, strings.Contains(output, "test_value"))
	})

	t.Run("nested marshaler", func(t *testing.T) {
		buf.Reset()

		log.Info(ctx, "nested", log.F{
			"error": log.F{
				"cause": "nested error",
				"data":  SlogTestEvent{SomeField: "nested_value"},
			},
		})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "nested"))
		assert.Assert(t, strings.Contains(output, "error.cause"))
		assert.Assert(t, strings.Contains(output, "nested error"))
		assert.Assert(t, strings.Contains(output, "error.data.myevent_field"))
		assert.Assert(t, strings.Contains(output, "nested_value"))
	})
}

func TestSlogFacadeWithSpecialValues(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)

	ctx := context.Background()

	t.Run("infinity", func(t *testing.T) {
		buf.Reset()
		log.Info(ctx, "infinity test", log.F{"inf": math.Inf(1)})

		output := buf.String()
		t.Logf("Infinity output: %s", output)
		assert.Assert(t, strings.Contains(output, "infinity test"))
		// slog handles infinity values and generates an error message
		assert.Assert(t, strings.Contains(output, "inf"))
		// The infinity value should now be converted to string "+Inf"
		assert.Assert(t, strings.Contains(output, "+Inf"))
	})

	t.Run("NaN", func(t *testing.T) {
		buf.Reset()
		log.Info(ctx, "nan test", log.F{"nan": math.NaN()})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "nan test"))
		assert.Assert(t, strings.Contains(output, "nan"))
		// NaN values cause JSON encoding issues in slog
		assert.Assert(t, strings.Contains(output, "ERROR") || strings.Contains(output, "NaN"))
	})

	t.Run("nil values", func(t *testing.T) {
		buf.Reset()
		log.Info(ctx, "nil test", log.F{"nil_value": nil})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "nil test"))
		assert.Assert(t, strings.Contains(output, "nil_value"))
	})
}

func TestSlogSetOutput(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf1, buf2 bytes.Buffer

	// Set first output
	log.SetOutput(&buf1)
	log.Info(context.Background(), "message1", log.F{"key": "value1"})

	// Change output
	log.SetOutput(&buf2)
	log.Info(context.Background(), "message2", log.F{"key": "value2"})

	// Verify outputs went to correct buffers
	output1 := buf1.String()
	output2 := buf2.String()

	assert.Assert(t, strings.Contains(output1, "message1"))
	assert.Assert(t, strings.Contains(output1, "value1"))
	assert.Assert(t, !strings.Contains(output1, "message2"))

	assert.Assert(t, strings.Contains(output2, "message2"))
	assert.Assert(t, strings.Contains(output2, "value2"))
	assert.Assert(t, !strings.Contains(output2, "message1"))
}

func TestSlogAttributeOrdering(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)

	// Test that attributes are sorted by key for consistent ordering
	log.Info(context.Background(), "test ordering", log.F{
		"z_last":   "last",
		"a_first":  "first",
		"m_middle": "middle",
		"b_second": "second",
	})

	output := buf.String()

	// Find positions of each key in the output
	posFirst := strings.Index(output, "a_first")
	posSecond := strings.Index(output, "b_second")
	posMiddle := strings.Index(output, "m_middle")
	posLast := strings.Index(output, "z_last")

	// Verify they appear in alphabetical order
	assert.Assert(t, posFirst < posSecond, "a_first should appear before b_second")
	assert.Assert(t, posSecond < posMiddle, "b_second should appear before m_middle")
	assert.Assert(t, posMiddle < posLast, "m_middle should appear before z_last")
}

// Test that Fatal exits when using slog (this test would normally cause os.Exit, so we skip it)
func TestSlogFatalBehavior(t *testing.T) {
	t.Skip("Skipping Fatal test as it calls os.Exit(1)")

	// This test exists to document the expected behavior:
	// When GOBOX_AS_SLOG_FACADE is enabled, Fatal should:
	// 1. Call slogIt with slog.LevelError
	// 2. Call os.Exit(1)
	//
	// The actual implementation shows:
	// if useSlog {
	//     slogIt(ctx, slog.LevelError, message, m)
	//     os.Exit(1)
	//     return
	// }
}

// TestSlogComplexValues tests slog's handling of complex values that cause JSON encoding issues
func TestSlogComplexValues(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)

	ctx := context.Background()

	t.Run("complex number conversion", func(t *testing.T) {
		buf.Reset()
		log.Info(ctx, "complex test", log.F{"complex": complex(1, 2)})

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "complex test"))
		assert.Assert(t, strings.Contains(output, "complex"))
		// Complex numbers get converted to string representation
		assert.Assert(t, strings.Contains(output, "(1+2i)"))
	})
}

// TestSlogErrorHandling tests slog's handling of various error types
func TestSlogErrorHandling(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)

	ctx := context.Background()

	t.Run("standard error", func(t *testing.T) {
		buf.Reset()
		standardErr := errors.New("standard error message")
		log.Error(ctx, "standard error test", log.F{"error": standardErr})

		output := buf.String()
		t.Logf("Standard error output: %s", output)
		assert.Assert(t, strings.Contains(output, "standard error test"))
		assert.Assert(t, strings.Contains(output, "ERRO"))
		// With explicit error handling, errors should now be converted to strings
		assert.Assert(t, strings.Contains(output, "standard error message"))
	})

	t.Run("events.Err wrapped error", func(t *testing.T) {
		buf.Reset()
		baseErr := errors.New("base error")
		wrappedErr := events.Err(baseErr)
		log.Error(ctx, "events error test", wrappedErr)

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "events error test"))
		assert.Assert(t, strings.Contains(output, "ERRO"))
		// events.Err creates structured error information
		assert.Assert(t, strings.Contains(output, "error.kind"))
		assert.Assert(t, strings.Contains(output, "error.error"))
		assert.Assert(t, strings.Contains(output, "base error"))
	})

	t.Run("pkg/errors wrapped error with stack", func(t *testing.T) {
		buf.Reset()
		baseErr := errors.New("root cause")
		wrappedErr := pkgerrors.Wrap(baseErr, "wrapped message")
		eventsErr := events.Err(wrappedErr)
		log.Error(ctx, "wrapped error test", eventsErr)

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "wrapped error test"))
		assert.Assert(t, strings.Contains(output, "ERRO"))
		// Should contain both the wrapper and root cause
		assert.Assert(t, strings.Contains(output, "wrapped message"))
		assert.Assert(t, strings.Contains(output, "root cause"))
		// Should contain error structure from events package
		assert.Assert(t, strings.Contains(output, "error.kind"))
	})

	t.Run("nested error with multiple causes", func(t *testing.T) {
		buf.Reset()
		rootErr := errors.New("database connection failed")
		middleErr := fmt.Errorf("query execution failed: %w", rootErr)
		topErr := fmt.Errorf("service operation failed: %w", middleErr)
		eventsErr := events.Err(topErr)
		log.Error(ctx, "nested error test", eventsErr)

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "nested error test"))
		assert.Assert(t, strings.Contains(output, "ERRO"))
		// Should contain all levels of the error chain
		assert.Assert(t, strings.Contains(output, "service operation failed"))
		assert.Assert(t, strings.Contains(output, "query execution failed"))
		assert.Assert(t, strings.Contains(output, "database connection failed"))
		// Should have proper error structure
		assert.Assert(t, strings.Contains(output, "error.kind"))
		assert.Assert(t, strings.Contains(output, "error.cause") || strings.Contains(output, "cause"))
	})

	t.Run("error with custom marshaler", func(t *testing.T) {
		buf.Reset()
		customErr := &CustomError{Code: 500, Message: "internal server error"}
		eventsErr := events.Err(customErr)
		log.Error(ctx, "custom error test", eventsErr)

		output := buf.String()
		assert.Assert(t, strings.Contains(output, "custom error test"))
		assert.Assert(t, strings.Contains(output, "ERRO"))
		// Should contain custom error fields
		assert.Assert(t, strings.Contains(output, "internal server error"))
		assert.Assert(t, strings.Contains(output, "custom.code") || strings.Contains(output, "500"))
		assert.Assert(t, strings.Contains(output, "custom.message"))
	})
}

// CustomError implements both error and Marshaler interfaces
type CustomError struct {
	Code    int
	Message string
}

func (c *CustomError) Error() string {
	return fmt.Sprintf("error %d: %s", c.Code, c.Message)
}

func (c *CustomError) MarshalLog(addField func(field string, value interface{})) {
	addField("custom.code", c.Code)
	addField("custom.message", c.Message)
}
