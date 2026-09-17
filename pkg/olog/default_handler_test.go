package olog

import (
	"context"
	"log/slog"
	"testing"
)

// recordingSink is a terminal slog.Handler that records everything it
// is handed, deliberately without any level filtering of its own.
type recordingSink struct {
	records *[]slog.Record
	attrs   []slog.Attr
}

func (s *recordingSink) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // Why: Handle's signature is fixed by the slog.Handler interface.
func (s *recordingSink) Handle(_ context.Context, r slog.Record) error {
	r.AddAttrs(s.attrs...)
	*s.records = append(*s.records, r)
	return nil
}

func (s *recordingSink) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &recordingSink{records: s.records, attrs: append(append([]slog.Attr{}, s.attrs...), attrs...)}
}

func (s *recordingSink) WithGroup(string) slog.Handler { return s }

// TestSinkHandlerKeepsLevelChain ensures an injected sink receives
// records through olog's leveling chain, rather than replacing it.
func TestSinkHandlerKeepsLevelChain(t *testing.T) {
	t.Cleanup(func() {
		SetSinkHandler(nil)
		SetGlobalLevel(slog.LevelInfo)
	})

	var got []slog.Record
	var gotOpts *slog.HandlerOptions
	SetSinkHandler(func(opts *slog.HandlerOptions) slog.Handler {
		gotOpts = opts
		return &recordingSink{records: &got}
	})

	lr := newRegistry()
	logger := NewWithHandler(createHandler(lr, &metadata{ModulePath: "testModuleName", PackagePath: "testPackageName"}))

	if gotOpts == nil {
		t.Fatal("expected sink factory to receive handler options")
	}
	if !gotOpts.AddSource {
		t.Error("expected AddSource to be set on the options handed to the sink")
	}
	if gotOpts.Level == nil {
		t.Error("expected the olog leveler to be handed to the sink")
	}

	// Registry gating is applied ahead of the sink.
	lr.Set(slog.LevelError, "testPackageName")
	logger.Info("suppressed by registry")
	if len(got) != 0 {
		t.Fatalf("expected registry level to gate the sink, got %d records", len(got))
	}

	lr.Set(slog.LevelDebug, "testPackageName")
	logger.Debug("passes registry")
	if len(got) != 1 {
		t.Fatalf("expected 1 record after lowering the registry level, got %d", len(got))
	}

	// LevelResolver overrides are applied ahead of the sink too.
	SetLevelResolver(func(context.Context, []string, slog.Level) (slog.Level, bool) {
		return slog.LevelError, true
	})
	t.Cleanup(func() { SetLevelResolver(nil) })

	logger.Debug("suppressed by resolver")
	if len(got) != 1 {
		t.Fatalf("expected resolver override to gate the sink, got %d records", len(got))
	}

	// Module attribution is still applied by olog, not the sink.
	SetLevelResolver(nil)
	moduleLogger := NewWithHandler(createHandler(lr, &metadata{ModulePath: "otherModule", ModuleVersion: "v1.2.3", PackagePath: "otherPackage"}))
	lr.Set(slog.LevelDebug, "otherPackage")
	moduleLogger.Info("attributed")

	last := got[len(got)-1]
	attrs := map[string]string{}
	last.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	if attrs["module"] != "otherModule" || attrs["modulever"] != "v1.2.3" {
		t.Errorf("expected module attribution on sink records, got %v", attrs)
	}
}

// TestSinkHandlerNilRestoresBuiltIn ensures passing nil restores the
// built-in sinks.
func TestSinkHandlerNilRestoresBuiltIn(t *testing.T) {
	SetSinkHandler(func(*slog.HandlerOptions) slog.Handler { return &recordingSink{records: &[]slog.Record{}} })
	SetSinkHandler(nil)

	if globalSink.Load() != nil {
		t.Fatal("expected SetSinkHandler(nil) to clear the installed sink")
	}
}
