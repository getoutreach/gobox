package number_test

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/number"
	"github.com/getoutreach/gobox/pkg/pointer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToInt64(t *testing.T) {
	value, err := number.ToInt64(1234)
	require.NoError(t, err)
	assert.Equal(t, int64(1234), value)
}

func TestToInt64Value(t *testing.T) {
	value, err := number.ToInt64Value[int](nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), value)

	value, err = number.ToInt64Value(pointer.ToPtr[int](1234))
	require.NoError(t, err)
	assert.Equal(t, int64(1234), value)
}

func TestToUInt64(t *testing.T) {
	tests := []struct {
		name           string
		arg            float64
		expectedErr    error
		expectedResult uint64
	}{
		{"Positive value", 42, nil, uint64(42)},
		{"Zero value", 0, nil, uint64(0)},
		{"Big value", 9223372036854775808, nil, uint64(9223372036854775808)},
		{"Negative value", -1, errors.New("value underflow when converting -1 to uint64"), uint64(0)},
		{
			"Float too big value",
			987654321012345678987654.2,
			errors.New("value overflow when converting 9.876543210123457e+23 to uint64"),
			uint64(0),
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := number.ToUInt64(tt.arg)
				if !errors.Is(err, tt.expectedErr) {
					if err.Error() != tt.expectedErr.Error() {
						t.Errorf("ToUInt64() error = '%v', want '%v'", err, tt.expectedErr)
					}
				}
				if got != tt.expectedResult {
					t.Errorf("ToUInt64() = '%v', want '%v'", got, tt.expectedResult)
				}
			},
		)
	}
}

func TestToUInt64Value(t *testing.T) {
	tests := []struct {
		name           string
		arg            *float64
		expectedErr    error
		expectedResult uint64
	}{
		{"Null", nil, nil, uint64(0)},
		{"Positive value", pointer.ToPtr[float64](42), nil, uint64(42)},
		{"Zero value", pointer.ToPtr[float64](0), nil, uint64(0)},
		{"Big value", pointer.ToPtr[float64](9223372036854775808), nil, uint64(9223372036854775808)},
		{"Negative value", pointer.ToPtr[float64](-1), errors.New("value underflow when converting -1 to uint64"), uint64(0)},
		{
			"Float too big value",
			pointer.ToPtr[float64](987654321012345678987654.2),
			errors.New("value overflow when converting 9.876543210123457e+23 to uint64"),
			uint64(0),
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := number.ToUInt64Value(tt.arg)
				if !errors.Is(err, tt.expectedErr) {
					if err.Error() != tt.expectedErr.Error() {
						t.Errorf("ToUInt64() error = '%v', want '%v'", err, tt.expectedErr)
					}
				}
				if got != tt.expectedResult {
					t.Errorf("ToUInt64() = '%v', want '%v'", got, tt.expectedResult)
				}
			},
		)
	}
}
