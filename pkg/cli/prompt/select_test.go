// Copyright 2026 Outreach Corporation. All Rights Reserved.

package prompt

import (
	"errors"
	"testing"
)

func TestSelectNoOptions(t *testing.T) {
	// No options is rejected before a Program is ever started, so this is
	// safe to call directly without a TTY. Select's other behavior is the
	// shared list model's, covered by list_test.go.
	if _, err := Select(t.Context(), SelectConfig{}); !errors.Is(err, ErrAborted) {
		t.Errorf("err = %v, want ErrAborted", err)
	}
}
