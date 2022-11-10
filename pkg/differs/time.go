// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides Comparers related to time
package differs

import "time"

// RFC3339Time allows RFC3339Time string to be matched against it
// when differs.Custom is passed to cmp
func RFC3339Time() CustomComparer {
	return Customf(func(o interface{}) bool {
		if s, ok := o.(string); ok {
			_, err := time.Parse(time.RFC3339, s)
			return err == nil
		}
		return false
	})
}

func RFC3339NanoTime() CustomComparer {
	return Customf(func(o interface{}) bool {
		if s, ok := o.(string); ok {
			_, err := time.Parse(time.RFC3339Nano, s)
			return err == nil
		}
		return false
	})
}
