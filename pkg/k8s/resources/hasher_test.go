package resources_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/k8s/resources"
)

type Spec struct {
	SP *string
	IP *int
	S  string
	I  int32
}

func TestHash(t *testing.T) {
	strVal := "val"
	iVal := 10
	tests := []struct {
		Name     string
		Expected string
		Spec     Spec
	}{
		{
			Name:     "defaults with nil",
			Expected: "silDF-ElsAHKby-f0m8JYgtFGZx-z033NCXn8CBJw0E=",
			Spec:     Spec{},
		},
		{
			Name:     "simple case",
			Expected: "UZX4stUw5WaRSSbJ90ueRCf5RDai6iHukt3HERZ7EZU=",
			Spec: Spec{
				SP: &strVal,
				IP: &iVal,
				S:  "other",
				I:  432,
			},
		},
	}

	for _, tt := range tests {
		hash, err := resources.Hash(tt.Spec)
		assert.NilError(t, err)
		// if hash all the sudden changes, it means:
		// * Hash method changed - please update the pre-calculated hashes above
		// * there is a non-deterministic algorithm somewhere, that produces
		//   OS or env specific hash. This is BAD because this can also potentially cause
		//   issues in production (hash is expected to be deterministic).
		//   Find the root cause and fix it (do not disable this test).
		assert.Equal(t, tt.Expected, hash, tt.Name)
	}
}
