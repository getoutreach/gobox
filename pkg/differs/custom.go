package differs

import "github.com/google/go-cmp/cmp"

// CustomComparer is the type returned by custom comparisons
type CustomComparer interface {
	CompareCustom(o interface{}) bool
}

func Custom() cmp.Option {
	return cmp.FilterValues(
		func(l, r interface{}) bool {
			_, lok := l.(CustomComparer)
			_, rok := r.(CustomComparer)
			return lok || rok
		},
		cmp.Comparer(func(l, r interface{}) bool {
			if lcmp, lok := l.(CustomComparer); lok {
				return lcmp.CompareCustom(r)
			}
			return r.(CustomComparer).CompareCustom(l)
		}),
	)
}

// Customf converts a function into a custom comparer
type Customf func(o interface{}) bool

func (c Customf) CompareCustom(o interface{}) bool {
	return c(o)
}
