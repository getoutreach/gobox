// Copyright 2025 Outreach Corporation. All Rights Reserved.
// Description: Numeric conversion utilities.

// Package number provides functions for converting numbers
package number

import (
	"math"

	"github.com/getoutreach/gobox/pkg/pointer"
	"golang.org/x/exp/constraints"

	"github.com/pkg/errors"
)

// Number can hold any numeric data.
type Number interface {
	constraints.Integer | constraints.Float
}

// ToInt64Value converts pointer to a numeric value to an int64.
func ToInt64Value[T Number](ptr *T) (int64, error) {
	return ToInt64(pointer.ToValue(ptr))
}

// ToInt64 converts a numeric value to an int64.
func ToInt64[T Number](value T) (int64, error) {
	const typeName = "int64"
	switch value := interface{}(value).(type) {
	case int:
		return int64(value), nil
	case uint:
		if value > math.MaxInt64 {
			return 0, overflowError(value, typeName)
		}
		return int64(value), nil
	case int8:
		return int64(value), nil
	case uint8:
		return int64(value), nil
	case int16:
		return int64(value), nil
	case uint16:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case uint32:
		return int64(value), nil
	case int64:
		return value, nil
	case uintptr:
		if value > math.MaxInt64 {
			return 0, overflowError(value, typeName)
		}
		return int64(value), nil
	case uint64:
		if value > math.MaxInt64 {
			return 0, overflowError(value, typeName)
		}
		return int64(value), nil
	case float32:
		if value < math.MinInt64 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxInt64 {
			return 0, overflowError(value, typeName)
		}
		return int64(value), nil
	case float64:
		if value < math.MinInt64 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxInt64 {
			return 0, overflowError(value, typeName)
		}
		return int64(value), nil
	default: // should never happen - there's no other types.
		return 0, errors.Errorf("unable to convert %v (%T) to int64", value, value)
	}
}

// ToUInt64Value converts pointer to a numeric value to an uint64.
func ToUInt64Value[T Number](ptr *T) (uint64, error) {
	return ToUInt64(pointer.ToValue(ptr))
}

// ToUInt64 converts a numeric value to an uint64.
func ToUInt64[T Number](value T) (uint64, error) {
	const typeName = "uint64"
	switch value := interface{}(value).(type) {
	case int:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint64(value), nil
	case uint:
		return uint64(value), nil
	case int8:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint64(value), nil
	case uint8:
		return uint64(value), nil
	case int16:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint64(value), nil
	case uint16:
		return uint64(value), nil
	case int32:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint64(value), nil
	case uint32:
		return uint64(value), nil
	case int64:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint64(value), nil
	case uint64:
		return value, nil
	case uintptr:
		return uint64(value), nil
	case float32:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxUint64 {
			return 0, overflowError(value, typeName)
		}
		return uint64(value), nil
	case float64:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxUint64 {
			return 0, overflowError(value, typeName)
		}
		return uint64(value), nil
	default: // should never happen - there's no other types.
		return 0, errors.Errorf("unable to convert %v (%#v) to uint64", value, value)
	}
}

// ToInt32Value converts pointer to a numeric value to an int32.
//
//nolint:golint // Why: not used for now, could be used in the future.
func ToInt32Value[T Number](ptr *T) (int32, error) {
	return ToInt32(pointer.ToValue(ptr))
}

// ToInt32 converts a numeric value to an int32.
//
//nolint:funlen,gocyclo // Why: the method may be long, but it's simple and readable. Splitting it would hurt readability.
func ToInt32[T Number](value T) (int32, error) {
	const typeName = "int32"
	switch value := interface{}(value).(type) {
	case int:
		if value < math.MinInt32 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case uint:
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case int8:
		return int32(value), nil
	case uint8:
		return int32(value), nil
	case int16:
		return int32(value), nil
	case uint16:
		return int32(value), nil
	case int32:
		return value, nil
	case uint32:
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case int64:
		if value < math.MinInt32 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case uint64:
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case uintptr:
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case float32:
		if value < math.MinInt32 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	case float64:
		if value < math.MinInt32 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxInt32 {
			return 0, overflowError(value, typeName)
		}
		return int32(value), nil
	default: // should never happen - there's no other types.
		return 0, errors.Errorf("unable to convert %#v to int32", value)
	}
}

// ToUInt32Value converts pointer to a numeric value to an uint32.
//
//nolint:golint // Why: not used for now, could be used in the future.
func ToUInt32Value[T Number](ptr *T) (uint32, error) {
	return ToUInt32(pointer.ToValue(ptr))
}

// ToUInt32 converts a numeric value to an uint32.
//
//nolint:funlen,gocyclo // Why: the method may be long, but it's simple and readable. Splitting it would hurt readability.
func ToUInt32[T Number](value T) (uint32, error) {
	const typeName = "uint32"
	switch value := interface{}(value).(type) {
	case int:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	case uint:
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	case int8:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint32(value), nil
	case uint8:
		return uint32(value), nil
	case int16:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint32(value), nil
	case uint16:
		return uint32(value), nil
	case int32:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		return uint32(value), nil
	case uint32:
		return value, nil
	case int64:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	case uint64:
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	case uintptr:
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	case float32:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	case float64:
		if value < 0 {
			return 0, underflowError(value, typeName)
		}
		if value > math.MaxUint32 {
			return 0, overflowError(value, typeName)
		}
		return uint32(value), nil
	default: // should never happen - there's no other types.
		return 0, errors.Errorf("unable to convert %#v to uint32", value)
	}
}

func underflowError[T Number](number T, typeName string) error {
	const underflowErrorMessage = "value underflow when converting %v to %v"
	return errors.Errorf(underflowErrorMessage, number, typeName)
}

func overflowError[T Number](number T, typeName string) error {
	const overflowErrorMessage = "value overflow when converting %v to %v"
	return errors.Errorf(overflowErrorMessage, number, typeName)
}
