// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides Comparers related to stack traces
package differs

import (
	"regexp"
	"strings"

	"github.com/google/go-cmp/cmp"
)

// nolint:gochecknoglobals // Why: regex used in multiple places
var stripNumbersRe = regexp.MustCompile(`:(\d*)`)

// StackLike allows a stacktrace to be matched against it
// when differs.Custom is passed to cmp
//
// The StackTrace() function is used for the actual matching
func StackLike(want string) CustomComparer {
	return Customf(func(o interface{}) bool {
		if got, ok := o.(string); ok {
			return StackTrace(want, got) == ""
		}
		return false
	})
}

// StackTrace returns the diff between two stack traces by comparing loosely
//
// In particular, the expected stacktrace can be shorter as well only have
// a substring.  This allows removing line numbers and memory addresses in
// the trace.
func StackTrace(want, got string) string {
	want, got = strings.TrimSpace(want), strings.TrimSpace(got)
	stringContains := cmp.Comparer(func(x, y string) bool {
		x, y = strings.TrimSpace(x), strings.TrimSpace(y)
		x = stripNumbersRe.ReplaceAllString(x, "")
		y = stripNumbersRe.ReplaceAllString(y, "")
		return strings.Contains(x, y) || strings.Contains(y, x)
	})
	wlines := strings.Split(want, "\n")
	glines := strings.Split(got, "\n")
	if len(glines) > len(wlines) {
		glines = glines[:len(wlines)]
	}
	return cmp.Diff(wlines, glines, stringContains)
}
