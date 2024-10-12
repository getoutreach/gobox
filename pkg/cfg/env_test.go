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
	if err != nil {
		t.Fatalf("failed to set environment variable: %v", err)
	}

	t.Run("if key not set; error is returned", func(t *testing.T) {
		v, err := EnvString(notsetKey)
		t.ErrorContains(t, err, notsetKey)
		t.ErrorContains(t, err, "environment variable not set")
	})

	t.Run("returns set value", func(t *testing.T) {
		v, err := EnvString(key)
		if err != nil {
			t.Fatalf("got unexpected error %s", err.Error())
		}

		if v != value {
			t.Fatalf("expected %s; got %s", value, v)
		}
	})
}

func TestEnvSecret(t *testing.T) {
	key := uuid.NewString()
	value := uuid.NewString()
	notsetKey := uuid.NewString()
	err := os.Setenv(key, value)
	if err != nil {
		t.Fatalf("failed to set environment variable: %v", err)
	}

	t.Run("if key not set; error is returned", func(t *testing.T) {
		v, err := EnvSecret(notsetKey)
		if err == nil {
			t.Fatalf("expected error fetching %s; got nil error and value %v", notsetKey, v)
		}
	})

	v, err := EnvSecret(key)
	if err != nil {
		t.Fatalf("got unexpected error %s", err.Error())
	}

	if string(v) != value {
		t.Fatalf("expected %s; got %s", value, v)
	}
}
