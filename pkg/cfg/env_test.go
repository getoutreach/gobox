package cfg

import (
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestEnvString(t *testing.T) {
	key := uuid.NewString()
	value := uuid.NewString()
	notsetKey := uuid.NewString()
	err := os.Setenv(key, value)
	assert.NilError(t, err, "failed to set environment variable")

	t.Run("if key not set; error is returned", func(t *testing.T) {
		v, err := EnvString(notsetKey)
		t.ErrorContains(t, err, notsetKey)
		t.ErrorContains(t, err, "environment variable not set")
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
		v, err := EnvSecret(notsetKey)
		t.ErrorContains(t, err, notsetKey)
		t.ErrorContains(t, err, "environment variable not set")
	})

	v, err := EnvSecret(key)
	assert.NilError(t, err)

	assert.Equal(t, string(v), value)
}
