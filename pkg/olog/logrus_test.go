package olog

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestTextLogger(t *testing.T) {
	buffer := bytes.Buffer{}
	logger := logrus.New()
	logger.SetFormatter(NewLogrusTextFormatter())
	logger.SetOutput(&buffer)
	logger.SetLevel(logrus.DebugLevel)
	cases := []struct {
		name     string
		expected string
		msg      string
		fields   logrus.Fields
		level    logrus.Level
	}{
		{
			name:     "simple message",
			expected: "INFO info\n",
			msg:      "info",
			fields:   nil,
			level:    logrus.InfoLevel,
		},
		{
			name:     "debug message",
			expected: "DEBU this is a debug message\n",
			msg:      "this is a debug message",
			fields:   nil,
			level:    logrus.DebugLevel,
		},
		{
			name:     "message with keyvals",
			expected: "INFO info key1=val1 key2=val2\n",
			msg:      "info",
			fields: logrus.Fields{
				"key1": "val1",
				"key2": "val2",
			},
			level: logrus.InfoLevel,
		},
		{
			name:     "error message with keyvals",
			expected: "ERRO info key1=val1 key2=val2\n",
			msg:      "info",
			fields: logrus.Fields{
				"key1": "val1",
				"key2": "val2",
			},
			level: logrus.ErrorLevel,
		},
		{
			name:     "error message with multiline",
			expected: "ERRO info\n  key1=\n  │ val1\n  │ val2\n",
			msg:      "info",
			fields: logrus.Fields{
				"key1": "val1\nval2",
			},
			level: logrus.ErrorLevel,
		},
		{
			name:     "error field",
			expected: "ERRO info key1=\"error value\"\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": errors.New("error value")},
			level:    logrus.ErrorLevel,
		},
		{
			name:     "struct field",
			expected: "ERRO info key1={foo:bar}\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": struct{ foo string }{foo: "bar"}},
			level:    logrus.ErrorLevel,
		},
		{
			name:     "struct field quoted",
			expected: "ERRO info key1=\"{foo:bar baz}\"\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": struct{ foo string }{foo: "bar baz"}},
			level:    logrus.ErrorLevel,
		},
		{
			name:     "slice of strings",
			expected: "ERRO info key1=\"[foo bar]\"\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": []string{"foo", "bar"}},
			level:    logrus.ErrorLevel,
		},
		{
			name:     "slice of structs",
			expected: "ERRO info key1=\"[{foo:bar} {foo:baz}]\"\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": []struct{ foo string }{{foo: "bar"}, {foo: "baz"}}},
			level:    logrus.ErrorLevel,
		},
		{
			name:     "slice of errors",
			expected: "ERRO info key1=\"[error value1 error value2]\"\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": []error{errors.New("error value1"), errors.New("error value2")}},
			level:    logrus.ErrorLevel,
		},
		{
			name:     "map of strings",
			expected: "ERRO info key1=\"map[baz:qux foo:bar]\"\n",
			msg:      "info",
			fields:   logrus.Fields{"key1": map[string]string{"foo": "bar", "baz": "qux"}},
			level:    logrus.ErrorLevel,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buffer.Reset()
			var l logrus.FieldLogger
			l = logger
			if c.fields != nil {
				l = l.WithFields(c.fields)
			}
			switch c.level { // nolint: exhaustive // Why: only testing certain levels
			case logrus.DebugLevel:
				l.Debug(c.msg)
			case logrus.InfoLevel:
				l.Info(c.msg)
			case logrus.WarnLevel:
				l.Warn(c.msg)
			case logrus.ErrorLevel:
				l.Error(c.msg)
			}
			bufS := buffer.String()
			assert.Assert(t, strings.HasSuffix(bufS, c.expected), cmp.Diff(bufS, c.expected))
		})
	}
}
