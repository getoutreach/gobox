// Copyright 2022 Outreach Corporation. All Rights Reserved.
//
// Description: error
package queue

import (
	"fmt"
)

// NewMaxCapacityError
func NewMaxCapacityError(capacity int) *MaxCapacityError {
	return &MaxCapacityError{
		capacity: capacity,
	}
}

var _ error = new(MaxCapacityError)

// MaxCapacityError
type MaxCapacityError struct {
	capacity int
}

// Error
func (e *MaxCapacityError) Error() string {
	return fmt.Sprintf("queue is full, capacity: %d", e.capacity)
}
