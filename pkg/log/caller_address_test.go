// Copyright 2026 Outreach Corporation. All Rights Reserved.

package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/internal/callerpkg"
	"github.com/getoutreach/gobox/pkg/olog"
)

// TestLevelRoutesToCallerPackage checks that the level registry address of a
// log.Debug call is the package that made the call, not pkg/log.
func TestLevelRoutesToCallerPackage(t *testing.T) {
	log.SetShouldUseSlog(true)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		olog.SetGlobalLevel(slog.LevelInfo)
		olog.SetLevel(slog.LevelInfo, callerpkg.Path, "github.com/getoutreach/gobox/pkg/log")
		log.SetShouldUseSlog(false)
	})

	olog.SetGlobalLevel(slog.LevelInfo)
	ctx := context.Background()

	// A level set on pkg/log must not enable DEBUG for the caller package.
	olog.SetLevel(slog.LevelDebug, "github.com/getoutreach/gobox/pkg/log")
	buf.Reset()
	callerpkg.EmitDebug(ctx, "must-not-appear")
	if bytes.Contains(buf.Bytes(), []byte("must-not-appear")) {
		t.Fatalf("pkg/log address enabled DEBUG for caller package: %s", buf.String())
	}

	// A level set on the caller package must enable DEBUG for it.
	olog.SetLevel(slog.LevelInfo, "github.com/getoutreach/gobox/pkg/log")
	olog.SetLevel(slog.LevelDebug, callerpkg.Path)
	buf.Reset()
	callerpkg.EmitDebug(ctx, "must-appear")
	if !bytes.Contains(buf.Bytes(), []byte("must-appear")) {
		t.Fatalf("caller package address did not enable DEBUG: %s", buf.String())
	}
}
