package differs

import "strings"

// AnyString allows any string to be matched against it when
// differs.Custom is passed to cmp
func AnyString() CustomComparer {
	return Customf(func(o interface{}) bool {
		_, ok := o.(string)
		return ok
	})
}

// CaptureString matches any string the first time it is used but
// on the second attempt, the string has to match exactly the same as
// the first one.
func CaptureString() CustomComparer {
	var matched string
	seen := false
	return Customf(func(o interface{}) bool {
		s, ok := o.(string)
		if !seen && ok {
			seen, matched = true, s
		}
		return ok && matched == s
	})
}

// ContainsString matches any string where the provided string is a substring
// of the string it is being matched against
func Contains(ss string) CustomComparer {
	return Customf(func(o interface{}) bool {
		s, ok := o.(string)
		return ok && strings.Contains(s, ss)
	})
}
