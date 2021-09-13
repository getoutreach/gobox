package pool_test

import (
	"fmt"
	"time"

	"gotest.tools/v3/assert/cmp"
)

func WithinDuration(x, y time.Time, delta time.Duration) cmp.Comparison {
	return func() cmp.Result {
		d := x.Sub(y)
		if d < 0 {
			d = -d
		}
		if d <= delta {
			return cmp.ResultSuccess
		}
		return cmp.ResultFailure(fmt.Sprintf("times %v and %v are not within allow expected:%v actual:%v ", x, y, delta, d))
	}
}

func InDelta(x, y, delta float64) cmp.Comparison {
	return func() cmp.Result {
		d := x - y
		if d < 0 {
			d = -d
		}
		if d <= delta {
			return cmp.ResultSuccess
		}
		return cmp.ResultFailure(fmt.Sprintf("times %v and %v are not within allow expected:%v actual:%v ", x, y, delta, d))
	}
}
