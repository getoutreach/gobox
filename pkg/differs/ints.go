// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements custom comparers for integers.
package differs

// AnyInt64 matches any value of type int64.
func AnyInt64() CustomComparer {
	return Customf(func(o interface{}) bool {
		_, ok := o.(int64)
		return ok
	})
}
