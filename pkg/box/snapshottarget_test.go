// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests SnapshotTarget's YAML contract with snapshots.yaml.

package box_test

import (
	"testing"

	"go.yaml.in/yaml/v3"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/box"
)

// TestSnapshotTargetPostRestoreServerSide checks the ordered list round-trips,
// since consumers apply the files in the order given.
func TestSnapshotTargetPostRestoreServerSide(t *testing.T) {
	var c box.SnapshotGenerateConfig
	assert.NilError(t, yaml.Unmarshal([]byte(`
targets:
  base:
    post_restore: ./post-restore/manifests.yaml
    post_restore_server_side:
      - ./post-restore/opentelemetry-operator.yaml
      - ./post-restore/otel-collectors.yaml
`), &c))

	target := c.Targets["base"]
	assert.Equal(t, target.PostRestore, "./post-restore/manifests.yaml")
	assert.DeepEqual(t, target.PostRestoreServerSide, []string{
		"./post-restore/opentelemetry-operator.yaml",
		"./post-restore/otel-collectors.yaml",
	})
}

// TestSnapshotTargetOmitted covers the common case: a target that does not use
// the field leaves it nil rather than failing to parse.
func TestSnapshotTargetOmitted(t *testing.T) {
	var c box.SnapshotGenerateConfig
	assert.NilError(t, yaml.Unmarshal([]byte(`
targets:
  base:
    post_restore: ./post-restore/manifests.yaml
`), &c))

	assert.Assert(t, c.Targets["base"].PostRestoreServerSide == nil)
}

// TestSnapshotTargetIgnoresUnknownKeys pins the lenient-parse behaviour that
// lets a snapshots.yaml using a newer field be read by an older consumer: the
// unknown key is skipped rather than erroring. Rollouts rely on this, so a
// switch to a strict decoder should be a deliberate decision.
func TestSnapshotTargetIgnoresUnknownKeys(t *testing.T) {
	var c box.SnapshotGenerateConfig
	assert.NilError(t, yaml.Unmarshal([]byte(`
targets:
  base:
    post_restore: ./post-restore/manifests.yaml
    some_field_from_the_future: [a, b]
`), &c))

	assert.Equal(t, c.Targets["base"].PostRestore, "./post-restore/manifests.yaml")
}
