package differs

// FloatRange allows a float value between the start and end
// when differs.Custom is passed to cmp
func FloatRange(start, end float64) CustomComparer {
	return Customf(func(o interface{}) bool {
		f, ok := o.(float64)
		return ok && f >= start && f <= end
	})
}
