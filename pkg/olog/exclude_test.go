package olog

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

type lazyValue struct{ v string }

func (l lazyValue) LogValue() slog.Value { return slog.StringValue(l.v) }

func loadExcludes(t *testing.T, yaml string) error {
	t.Helper()
	p := filepath.Join(t.TempDir(), "olog.yaml")
	assert.NilError(t, os.WriteFile(p, []byte(yaml), 0o644))
	t.Cleanup(func() { excludeRules.Store(nil) })
	return ConfigureFromFile(p)
}

func newExcludeTestLogger(t *testing.T, pkg string) (*slog.Logger, *testLogCapturer) {
	t.Helper()
	tc := NewTestCapturer(t)
	SetDefaultHandler(JSONHandler)
	SetGlobalLevel(slog.LevelDebug)
	return NewWithHandler(createHandler(globalLevelRegistry, &metadata{ModulePath: "mod", PackagePath: pkg})), tc
}

func TestExclude_Message(t *testing.T) {
	assert.NilError(t, loadExcludes(t, "exclude:\n  - message: '^health ?check'\n"))
	l, tc := newExcludeTestLogger(t, "pkg")

	l.Info("healthcheck ok")
	l.Error("health check failed") // dropped at any level
	l.Info("real work")

	logs := tc.GetLogs()
	assert.Equal(t, len(logs), 1)
	assert.Equal(t, logs[0].Message, "real work")
}

func TestExclude_Attrs(t *testing.T) {
	assert.NilError(t, loadExcludes(t, `exclude:
  - attrs:
      http.route: '^/healthz$'
      probe: 'true'
`))
	l, tc := newExcludeTestLogger(t, "pkg")

	// Record attrs, grouped key, LogValuer.
	l.Info("a", slog.Group("http", slog.String("route", "/healthz")), slog.String("probe", "true"))
	l.Info("b", slog.Group("http", slog.String("route", "/healthz")), slog.Any("probe", lazyValue{"true"}))
	// Only one attr matches: kept.
	l.Info("c", slog.Group("http", slog.String("route", "/healthz")))
	l.Info("d", slog.Group("http", slog.String("route", "/other")), slog.String("probe", "true"))
	// Bound via WithAttrs and WithGroup.
	l.With("probe", "true").WithGroup("http").Info("e", "route", "/healthz")
	l.WithGroup("http").With("route", "/healthz").With(slog.String("probe", "true")).Info("f")

	var msgs []string
	for _, e := range tc.GetLogs() {
		msgs = append(msgs, e.Message)
	}
	// "e": probe bound before the group => "probe", route in group => "http.route": dropped.
	// "f": probe bound inside group => "http.probe": does not match "probe", kept.
	assert.DeepEqual(t, msgs, []string{"c", "d", "f"})
}

func TestExclude_AddressScope(t *testing.T) {
	assert.NilError(t, loadExcludes(t, "exclude:\n  - address: noisy/pkg\n    message: spam\n"))

	l, tc := newExcludeTestLogger(t, "noisy/pkg")
	l.Info("spam")
	assert.Equal(t, len(tc.GetLogs()), 0)

	l, tc = newExcludeTestLogger(t, "other/pkg")
	l.Info("spam")
	assert.Equal(t, len(tc.GetLogs()), 1)
}

func TestExclude_InvalidRulesReportedValidApplied(t *testing.T) {
	err := loadExcludes(t, `exclude:
  - message: '('
  - address: x
  - message: spam
`)
	assert.Assert(t, errors.Is(err, ErrInvalidExclude))
	l, tc := newExcludeTestLogger(t, "pkg")
	l.Info("spam")
	l.Info("ok")
	assert.Equal(t, len(tc.GetLogs()), 1)
}

func TestExclude_ReloadReplacesRules(t *testing.T) {
	p := filepath.Join(t.TempDir(), "olog.yaml")
	t.Cleanup(func() { excludeRules.Store(nil) })
	l, tc := newExcludeTestLogger(t, "pkg")

	assert.NilError(t, os.WriteFile(p, []byte("exclude:\n  - message: spam\n"), 0o644))
	assert.NilError(t, ConfigureFromFile(p))
	l.Info("spam")
	assert.Equal(t, len(tc.GetLogs()), 0)

	assert.NilError(t, os.WriteFile(p, []byte("log: []\n"), 0o644))
	assert.NilError(t, ConfigureFromFile(p))
	l.Info("spam")
	assert.Equal(t, len(tc.GetLogs()), 1)
}

func BenchmarkExclude(b *testing.B) {
	SetDefaultHandler(JSONHandler)
	for name, rules := range map[string][]ExcludeConfig{
		"NoRules":     nil,
		"MessageRule": {{Message: "^health"}},
		"AttrRule":    {{Attrs: map[string]string{"http.route": "^/healthz$"}}},
	} {
		b.Run(name, func(b *testing.B) {
			cr, _ := compileExcludes(rules, "")
			excludeRules.Store(&cr)
			defer excludeRules.Store(nil)
			h := newExcludeHandler(slog.NewJSONHandler(io.Discard, nil), []string{"pkg"})
			l := slog.New(h)
			b.ReportAllocs()
			for b.Loop() {
				l.Info("work", slog.Group("http", slog.String("route", "/x")))
			}
		})
	}
}
