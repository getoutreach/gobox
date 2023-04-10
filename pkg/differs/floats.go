// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides Comparers related to floats
package differs

// FloatRange allows a float value between the start and end
// when differs.Custom is passed to cmp
func FloatRange(start, end float64) CustomComparer {
	return Customf(func(o interface{}) bool {
		f, ok := o.(float64)
		return ok && f >= start && f <= end
	})
}

// AnyFloat64 allows any float64 value
func AnyFloat64() CustomComparer {
	return Customf(func(o interface{}) bool {
		_, ok := o.(float64)
		return ok
	})
}
