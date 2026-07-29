// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: This file demonstrates set.Set usage.
package set_test

import (
	"fmt"

	"github.com/getoutreach/gobox/pkg/set"
)

func Example_construct() {
	s := set.Of("x", "y", "z")
	fmt.Println(s.Len())
	// Output: 3
}

func Example_membership() {
	s := set.Of("x", "y")
	fmt.Println(s.Contains("x"))
	fmt.Println(s.Contains("z"))
	// Output:
	// true
	// false
}

func Example_algebra() {
	a := set.Of("x", "y")
	b := set.Of("y", "z")
	fmt.Println(set.Sorted(a.Union(b)))
	fmt.Println(set.Sorted(a.Intersection(b)))
	fmt.Println(set.Sorted(a.Difference(b)))
	// Output:
	// [x y z]
	// [y]
	// [x]
}

func Example_iterate() {
	s := set.Of("x")
	for v := range s.All() {
		fmt.Println(v)
	}
	// Output: x
}
