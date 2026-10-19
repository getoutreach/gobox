// Copyright 2026 Outreach Corporation. All Rights Reserved.

package otelmetrics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/getoutreach/gobox/pkg/otelmetrics"
)

func TestConfigWithDefaultsAppliesServiceNamePushWorkerCountAndEmitBufferSize(t *testing.T) {
	cfg := otelmetrics.Config{}.WithDefaults("fallback-service")

	assert.Equal(t, "fallback-service", cfg.ServiceName)
	assert.Equal(t, otelmetrics.DefaultPushWorkerCount, cfg.PushWorkerCount)
	assert.Equal(t, otelmetrics.DefaultEmitBufferSize, cfg.EmitBufferSize)
}

func TestConfigWithDefaultsPreservesExplicitValues(t *testing.T) {
	cfg := otelmetrics.Config{
		ServiceName:     "explicit-service",
		PushWorkerCount: 7,
		EmitBufferSize:  42,
	}.WithDefaults("fallback-service")

	assert.Equal(t, "explicit-service", cfg.ServiceName)
	assert.Equal(t, 7, cfg.PushWorkerCount)
	assert.Equal(t, 42, cfg.EmitBufferSize)
}

// TestConfigWithDefaultsNeverLeavesZeroPushWorkerCount guards against the
// regression that shipped alongside the bounded emit queue: with zero
// workers, nothing ever drains the queue, so every Record() call finds it
// "full" and silently drops - metrics recording is completely disabled by
// default unless PushWorkerCount happens to be set in every deployment's
// config.
func TestConfigWithDefaultsNeverLeavesZeroPushWorkerCount(t *testing.T) {
	for _, in := range []int{0, -1, -100} {
		cfg := otelmetrics.Config{PushWorkerCount: in}.WithDefaults("svc")
		assert.Positive(t, cfg.PushWorkerCount, "input PushWorkerCount=%d", in)
	}
}

// TestConfigWithDefaultsNeverLeavesZeroEmitBufferSize guards against a
// zero-capacity emit queue, which bounds outstanding emits down to zero and
// makes every Record() drop, since sending on an unbuffered channel only
// succeeds in exact lockstep with a worker's receive.
func TestConfigWithDefaultsNeverLeavesZeroEmitBufferSize(t *testing.T) {
	for _, in := range []int{0, -1, -100} {
		cfg := otelmetrics.Config{EmitBufferSize: in}.WithDefaults("svc")
		assert.Positive(t, cfg.EmitBufferSize, "input EmitBufferSize=%d", in)
	}
}
