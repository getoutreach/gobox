package cfg

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
)

func TestEnvString(t *testing.T) {
	key := uuid.NewString()
	value := uuid.NewString()
	notsetKey := uuid.NewString()
	err := os.Setenv(key, value)
	assert.NilError(t, err, "failed to set environment variable")

	t.Run("if key not set; error is returned", func(t *testing.T) {
		_, err := EnvString(notsetKey)
		assert.ErrorContains(t, err, notsetKey)
		assert.ErrorContains(t, err, "environment variable not set")
	})

	t.Run("returns set value", func(t *testing.T) {
		v, err := EnvString(key)
		assert.NilError(t, err)

		assert.Equal(t, v, value)
	})
}

func TestEnvSecret(t *testing.T) {
	key := uuid.NewString()
	value := uuid.NewString()
	notsetKey := uuid.NewString()
	err := os.Setenv(key, value)
	assert.NilError(t, err, "failed to set environment variable")

	t.Run("if key not set; error is returned", func(t *testing.T) {
		_, err := EnvSecret(notsetKey)
		assert.ErrorContains(t, err, notsetKey)
		assert.ErrorContains(t, err, "environment variable not set")
	})

	v, err := EnvSecret(key)
	assert.NilError(t, err)

	assert.Equal(t, string(v), value)
}
