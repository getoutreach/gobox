// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests SnapshotTarget's YAML contract with snapshots.yaml.

package box_test

import (
	"testing"

	"go.yaml.in/yaml/v3"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/box"
)

// TestSnapshotTargetPostRestoreServerSide checks the ordered list round-trips.
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

// TestSnapshotTargetOmitted checks an unset field stays nil.
func TestSnapshotTargetOmitted(t *testing.T) {
	var c box.SnapshotGenerateConfig
	assert.NilError(t, yaml.Unmarshal([]byte(`
targets:
  base:
    post_restore: ./post-restore/manifests.yaml
`), &c))

	assert.Assert(t, c.Targets["base"].PostRestoreServerSide == nil)
}

// TestSnapshotTargetIgnoresUnknownKeys pins the lenient parse that lets an older
// consumer read a newer snapshots.yaml. A strict decoder should be a deliberate choice.
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
