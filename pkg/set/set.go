// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Implements the set package.

// Package set provides a generic set for comparable types.
//
// Set's underlying type is map[T]struct{} -- the same representation the Go
// working group chose for the proposed container/set.Set (golang/go#69230) --
// so it can be ranged over and used anywhere a map is accepted, and an
// existing map[T]struct{} (or a named type over it) converts to a Set at zero
// cost. The zero value supports read operations (Contains, Len, All, Equal,
// String); construct via Of, Collect, or make before calling a mutator, the
// same as an uninitialized Go map:
//
//	var z Set[string]
//	z.Contains("x") // false, no panic
//	z.Insert("x")   // panics: assignment to entry in nil map
//
// Set is not safe for concurrent use.
//
// Coming from deckarep/golang-set, the migration is largely a rename:
//
//	Add          -> Insert
//	Remove       -> Delete
//	Cardinality  -> Len
//	Iter/Each    -> All
package set

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
)

// Set is a generic set of comparable elements, backed by a map[T]struct{}.
// See the package doc for the nil-safety rules: reads are nil-safe, but
// mutators that grow the set panic on a nil Set, just like an uninitialized
// map.
type Set[T comparable] map[T]struct{}

// Of returns a new Set containing the given values.
func Of[T comparable](v ...T) Set[T] {
	s := make(Set[T], len(v))
	for _, x := range v {
		s[x] = struct{}{}
	}
	return s
}

// Collect returns a new Set containing the values produced by seq,
// deduplicating them.
func Collect[T comparable](seq iter.Seq[T]) Set[T] {
	s := make(Set[T])
	for v := range seq {
		s[v] = struct{}{}
	}
	return s
}

// Insert adds v to s and reports whether the set grew (i.e. v was not
// already present). Insert panics if s is nil, the same as assigning into a
// nil map.
func (s Set[T]) Insert(v T) bool {
	if _, ok := s[v]; ok {
		return false
	}
	s[v] = struct{}{}
	return true
}

// InsertAll adds every value produced by seq to s and reports whether the
// set grew. InsertAll panics if s is nil and seq produces at least one
// value.
func (s Set[T]) InsertAll(seq iter.Seq[T]) bool {
	grew := false
	for v := range seq {
		if s.Insert(v) {
			grew = true
		}
	}
	return grew
}

// Delete removes v from s and reports whether it was present. Delete is a
// nil-safe no-op (not a panic) if s is nil, since it only ever removes.
func (s Set[T]) Delete(v T) bool {
	if _, ok := s[v]; !ok {
		return false
	}
	delete(s, v)
	return true
}

// DeleteAll removes every value produced by seq from s and reports whether
// the set shrank. DeleteAll is a nil-safe no-op if s is nil.
func (s Set[T]) DeleteAll(seq iter.Seq[T]) bool {
	shrank := false
	for v := range seq {
		if s.Delete(v) {
			shrank = true
		}
	}
	return shrank
}

// DeleteFunc removes every element for which f reports true and reports
// whether the set shrank. DeleteFunc is a nil-safe no-op if s is nil.
func (s Set[T]) DeleteFunc(f func(T) bool) bool {
	before := len(s)
	maps.DeleteFunc(s, func(v T, _ struct{}) bool { return f(v) })
	return len(s) != before
}

// Contains reports whether v is a member of s. Contains is nil-safe.
func (s Set[T]) Contains(v T) bool {
	_, ok := s[v]
	return ok
}

// ContainsAll reports whether every value produced by seq is a member of s.
// ContainsAll is nil-safe.
func (s Set[T]) ContainsAll(seq iter.Seq[T]) bool {
	for v := range seq {
		if !s.Contains(v) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether any value produced by seq is a member of s.
// ContainsAny is nil-safe.
func (s Set[T]) ContainsAny(seq iter.Seq[T]) bool {
	for v := range seq {
		if s.Contains(v) {
			return true
		}
	}
	return false
}

// Len returns the number of elements in s. Len is nil-safe.
func (s Set[T]) Len() int {
	return len(s)
}

// Clear removes all elements from s. Clear is a nil-safe no-op if s is nil.
func (s Set[T]) Clear() {
	clear(s)
}

// Clone returns a shallow copy of s. Unlike maps.Clone, Clone always returns
// a non-nil set, even if s is nil.
func (s Set[T]) Clone() Set[T] {
	if s == nil {
		return make(Set[T])
	}
	return maps.Clone(s)
}

// String returns a "{a, b, c}"-style representation of s. Elements are
// sorted by their fmt.Sprint representation, so the result is deterministic
// (though not necessarily in numeric order for numeric T). String is
// nil-safe.
func (s Set[T]) String() string {
	elems := make([]string, 0, len(s))
	for v := range s {
		elems = append(elems, fmt.Sprint(v))
	}
	slices.Sort(elems)
	return "{" + strings.Join(elems, ", ") + "}"
}

// All returns an iterator over the elements of s, in unspecified order. All
// is nil-safe.
func (s Set[T]) All() iter.Seq[T] {
	return maps.Keys(s)
}

// Union returns a new set containing every element of s and o.
func (s Set[T]) Union(o Set[T]) Set[T] {
	r := make(Set[T], len(s)+len(o))
	maps.Copy(r, s)
	maps.Copy(r, o)
	return r
}

// UnionWith adds every element of o to s in place. UnionWith panics if s is
// nil and o is non-empty.
func (s Set[T]) UnionWith(o Set[T]) {
	maps.Copy(s, o)
}

// Intersection returns a new set containing the elements present in both s
// and o.
func (s Set[T]) Intersection(o Set[T]) Set[T] {
	small, big := s, o
	if len(o) < len(s) {
		small, big = o, s
	}
	r := make(Set[T], len(small))
	for v := range small {
		if _, ok := big[v]; ok {
			r[v] = struct{}{}
		}
	}
	return r
}

// IntersectionWith removes every element of s that is not also in o.
// IntersectionWith is nil-safe since it only ever removes.
func (s Set[T]) IntersectionWith(o Set[T]) {
	maps.DeleteFunc(s, func(v T, _ struct{}) bool {
		_, ok := o[v]
		return !ok
	})
}

// Difference returns a new set containing the elements of s that are not in
// o.
func (s Set[T]) Difference(o Set[T]) Set[T] {
	r := make(Set[T])
	for v := range s {
		if _, ok := o[v]; !ok {
			r[v] = struct{}{}
		}
	}
	return r
}

// DifferenceWith removes every element of o from s. DifferenceWith is
// nil-safe since it only ever removes.
func (s Set[T]) DifferenceWith(o Set[T]) {
	for v := range o {
		delete(s, v)
	}
}

// SymmetricDifference returns a new set containing the elements that are in
// exactly one of s or o.
func (s Set[T]) SymmetricDifference(o Set[T]) Set[T] {
	r := make(Set[T])
	for v := range s {
		if _, ok := o[v]; !ok {
			r[v] = struct{}{}
		}
	}
	for v := range o {
		if _, ok := s[v]; !ok {
			r[v] = struct{}{}
		}
	}
	return r
}

// SymmetricDifferenceWith updates s in place to contain the elements that
// are in exactly one of s or o. SymmetricDifferenceWith panics if s is nil
// and o is non-empty.
func (s Set[T]) SymmetricDifferenceWith(o Set[T]) {
	for v := range o {
		if s.Contains(v) {
			delete(s, v)
		} else {
			s.Insert(v)
		}
	}
}

// Intersects reports whether s and o share at least one element. Intersects
// is nil-safe.
func (s Set[T]) Intersects(o Set[T]) bool {
	small, big := s, o
	if len(o) < len(s) {
		small, big = o, s
	}
	for v := range small {
		if _, ok := big[v]; ok {
			return true
		}
	}
	return false
}

// Equal reports whether s and o contain the same elements. Equal is
// nil-safe.
func (s Set[T]) Equal(o Set[T]) bool {
	return maps.Equal(s, o)
}

// Slice returns the elements of s as a slice, in unspecified order. It
// returns nil if s is empty. Slice is nil-safe.
func (s Set[T]) Slice() []T {
	return slices.Collect(s.All())
}

// Subset reports whether every element of a is also in b.
func Subset[T comparable](a, b Set[T]) bool {
	if len(a) > len(b) {
		return false
	}
	for v := range a {
		if _, ok := b[v]; !ok {
			return false
		}
	}
	return true
}

// Superset reports whether every element of b is also in a.
func Superset[T comparable](a, b Set[T]) bool {
	return Subset(b, a)
}

// Sorted returns the elements of s as an ascending sorted slice. It returns
// nil if s is empty.
func Sorted[T cmp.Ordered](s Set[T]) []T {
	return slices.Sorted(s.All())
}
