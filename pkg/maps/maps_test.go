package maps

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestMergeNoOverwrite(t *testing.T) {
	a := map[string]string{
		"a": "banana",
		"b": "apple",
	}
	b := map[string]string{
		"b": "pear",
		"c": "strawberry",
	}
	r := Merge(a, b, false)
	assert.Equal(t, "banana", r["a"])
	assert.Equal(t, "apple", r["b"])
	assert.Equal(t, "strawberry", r["c"])
}

func TestMergeWithOverwrite(t *testing.T) {
	a := map[string]bool{
		"a": true,
		"b": false,
	}
	b := map[string]bool{
		"b": true,
		"c": false,
	}
	r := Merge(a, b, true)
	assert.Equal(t, true, r["a"])
	assert.Equal(t, true, r["b"])
	assert.Equal(t, false, r["c"])
}
