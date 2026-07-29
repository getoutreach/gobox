package set

import (
	"maps"
	"slices"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
)

func TestOf(t *testing.T) {
	s := Of("a", "b", "b", "c")
	assert.Equal(t, 3, s.Len())
	assert.Assert(t, s.Contains("a"))
	assert.Assert(t, s.Contains("b"))
	assert.Assert(t, s.Contains("c"))
	assert.Assert(t, !s.Contains("d"))
}

func TestCollect(t *testing.T) {
	s := Collect(slices.Values([]int{1, 2, 2, 3}))
	assert.Equal(t, 3, s.Len())
	assert.DeepEqual(t, []int{1, 2, 3}, Sorted(s))
}

func TestInsert(t *testing.T) {
	var s Set[string]
	assert.Assert(t, s.Insert("a"))
	assert.Assert(t, !s.Insert("a"))
	assert.Assert(t, s.Insert("b"))
	assert.Equal(t, 2, s.Len())
}

func TestInsertNilReceiver(t *testing.T) {
	var s Set[string]
	assert.Assert(t, s == nil)
	s.Insert("a")
	assert.Assert(t, s != nil)
	assert.Assert(t, s.Contains("a"))
}

func TestInsertAll(t *testing.T) {
	s := Of("a")
	grew := s.InsertAll(slices.Values([]string{"a", "b", "c"}))
	assert.Assert(t, grew)
	assert.Equal(t, 3, s.Len())

	grew = s.InsertAll(slices.Values([]string{"a", "b", "c"}))
	assert.Assert(t, !grew)
}

func TestDelete(t *testing.T) {
	s := Of("a", "b")
	assert.Assert(t, s.Delete("a"))
	assert.Assert(t, !s.Delete("a"))
	assert.Equal(t, 1, s.Len())
}

func TestDeleteOnNilSet(t *testing.T) {
	var s Set[string]
	assert.Assert(t, !s.Delete("a"))
}

func TestDeleteAll(t *testing.T) {
	s := Of("a", "b", "c")
	shrank := s.DeleteAll(slices.Values([]string{"a", "b"}))
	assert.Assert(t, shrank)
	assert.Equal(t, 1, s.Len())

	shrank = s.DeleteAll(slices.Values([]string{"a"}))
	assert.Assert(t, !shrank)
}

func TestDeleteFunc(t *testing.T) {
	s := Of(1, 2, 3, 4, 5)
	shrank := s.DeleteFunc(func(v int) bool { return v%2 == 0 })
	assert.Assert(t, shrank)
	assert.DeepEqual(t, []int{1, 3, 5}, Sorted(s))
}

func TestContains(t *testing.T) {
	s := Of("a", "b")
	assert.Assert(t, s.Contains("a"))
	assert.Assert(t, !s.Contains("z"))
}

func TestContainsAll(t *testing.T) {
	s := Of("a", "b", "c")
	assert.Assert(t, s.ContainsAll(slices.Values([]string{"a", "b"})))
	assert.Assert(t, !s.ContainsAll(slices.Values([]string{"a", "z"})))
	assert.Assert(t, s.ContainsAll(slices.Values([]string{})))
}

func TestContainsAny(t *testing.T) {
	s := Of("a", "b", "c")
	assert.Assert(t, s.ContainsAny(slices.Values([]string{"z", "a"})))
	assert.Assert(t, !s.ContainsAny(slices.Values([]string{"y", "z"})))
	assert.Assert(t, !s.ContainsAny(slices.Values([]string{})))
}

func TestLen(t *testing.T) {
	assert.Equal(t, 0, Set[string]{}.Len())
	assert.Equal(t, 2, Of("a", "b").Len())
}

func TestClear(t *testing.T) {
	s := Of("a", "b")
	s.Clear()
	assert.Equal(t, 0, s.Len())
}

func TestClone(t *testing.T) {
	s := Of("a", "b")
	c := s.Clone()
	assert.Assert(t, s.Equal(c))
	c.Insert("z")
	assert.Assert(t, !s.Contains("z"))
}

func TestCloneNil(t *testing.T) {
	var s Set[string]
	c := s.Clone()
	assert.Assert(t, c != nil)
	assert.Equal(t, 0, c.Len())
}

func TestString(t *testing.T) {
	s := Of("a")
	assert.Equal(t, "{a}", s.String())
	assert.Equal(t, "{}", Set[string]{}.String())
}

func TestStringSortedDeterministic(t *testing.T) {
	s := Of("c", "a", "b")
	assert.Equal(t, "{a, b, c}", s.String())
	assert.Equal(t, s.String(), s.String())
}

func TestAll(t *testing.T) {
	s := Of(1, 2, 3)
	var got []int
	for v := range s.All() {
		got = append(got, v)
	}
	slices.Sort(got)
	assert.DeepEqual(t, []int{1, 2, 3}, got)
}

func TestUnion(t *testing.T) {
	a := Of("a", "b")
	b := Of("b", "c")
	u := a.Union(b)
	assert.DeepEqual(t, []string{"a", "b", "c"}, Sorted(u))
	// Pure: operands unchanged.
	assert.Equal(t, 2, a.Len())
	assert.Equal(t, 2, b.Len())
}

func TestUnionSelf(t *testing.T) {
	a := Of("a", "b")
	u := a.Union(a)
	assert.Assert(t, u.Equal(a))
}

func TestUnionWith(t *testing.T) {
	a := Of("a", "b")
	b := Of("b", "c")
	a.UnionWith(b)
	assert.DeepEqual(t, []string{"a", "b", "c"}, Sorted(a))
}

func TestUnionWithNilReceiver(t *testing.T) {
	var s Set[string]
	s.UnionWith(Of("a", "b"))
	assert.DeepEqual(t, []string{"a", "b"}, Sorted(s))
}

func TestIntersection(t *testing.T) {
	a := Of("a", "b", "c")
	b := Of("b", "c", "d")
	assert.DeepEqual(t, []string{"b", "c"}, Sorted(a.Intersection(b)))
	assert.DeepEqual(t, []string(nil), Sorted(Of("a").Intersection(Of("b"))))
}

func TestIntersectionWith(t *testing.T) {
	a := Of("a", "b", "c")
	b := Of("b", "c", "d")
	a.IntersectionWith(b)
	assert.DeepEqual(t, []string{"b", "c"}, Sorted(a))
}

func TestDifference(t *testing.T) {
	a := Of("a", "b", "c")
	b := Of("b", "c", "d")
	assert.DeepEqual(t, []string{"a"}, Sorted(a.Difference(b)))
	assert.DeepEqual(t, []string(nil), Sorted(Of("a").Difference(Of("a"))))
}

func TestDifferenceWith(t *testing.T) {
	a := Of("a", "b", "c")
	b := Of("b", "c", "d")
	a.DifferenceWith(b)
	assert.DeepEqual(t, []string{"a"}, Sorted(a))
}

func TestSymmetricDifference(t *testing.T) {
	a := Of("a", "b", "c")
	b := Of("b", "c", "d")
	assert.DeepEqual(t, []string{"a", "d"}, Sorted(a.SymmetricDifference(b)))
}

func TestSymmetricDifferenceWith(t *testing.T) {
	a := Of("a", "b", "c")
	b := Of("b", "c", "d")
	a.SymmetricDifferenceWith(b)
	assert.DeepEqual(t, []string{"a", "d"}, Sorted(a))
}

func TestSymmetricDifferenceWithNilReceiver(t *testing.T) {
	var s Set[string]
	s.SymmetricDifferenceWith(Of("a", "b"))
	assert.DeepEqual(t, []string{"a", "b"}, Sorted(s))
}

func TestIntersects(t *testing.T) {
	assert.Assert(t, Of("a", "b").Intersects(Of("b", "c")))
	assert.Assert(t, !Of("a").Intersects(Of("b")))
	x := Of("a")
	assert.Assert(t, x.Intersects(x))
}

func TestEqual(t *testing.T) {
	assert.Assert(t, Of("a", "b").Equal(Of("b", "a")))
	assert.Assert(t, !Of("a", "b").Equal(Of("a")))
	assert.Assert(t, !Of("a", "b").Equal(Of("a", "c")))
}

func TestSlice(t *testing.T) {
	s := Of("a", "b", "c")
	got := s.Slice()
	slices.Sort(got)
	assert.DeepEqual(t, []string{"a", "b", "c"}, got)
}

func TestSubsetSuperset(t *testing.T) {
	a := Of("a", "b")
	b := Of("a", "b", "c")
	assert.Assert(t, Subset(a, b))
	assert.Assert(t, !Subset(b, a))
	assert.Assert(t, Superset(b, a))
	assert.Assert(t, !Superset(a, b))
	assert.Assert(t, Subset(a, a))
	assert.Assert(t, Superset(a, a))
}

func TestSorted(t *testing.T) {
	s := Of(3, 1, 2)
	assert.DeepEqual(t, []int{1, 2, 3}, Sorted(s))
	assert.DeepEqual(t, []int(nil), Sorted(Set[int]{}))
}

// TestNilZeroValueReads validates that every read method on a nil Set
// behaves as if the set were empty.
func TestNilZeroValueReads(t *testing.T) {
	var s Set[string]
	assert.Assert(t, !s.Contains("a"))
	assert.Equal(t, 0, s.Len())
	assert.DeepEqual(t, []string(nil), s.Slice())
	var count int
	for range s.All() {
		count++
	}
	assert.Equal(t, 0, count)
}

// TestZeroCostConversion validates that an existing map[T]struct{} (or a
// named type over it) converts to a Set at zero cost.
func TestZeroCostConversion(t *testing.T) {
	type FooSet map[string]struct{}
	existing := FooSet{"a": {}, "b": {}}
	s := Set[string](existing)
	assert.Equal(t, 2, s.Len())
	assert.Assert(t, s.Contains("a"))
}

// TestBoolBackedLegacySet validates the documented one-time rebuild path for
// a map[K]bool legacy set, which cannot convert at zero cost.
func TestBoolBackedLegacySet(t *testing.T) {
	legacy := map[int]bool{1: true, 2: true, 3: true}
	s := Collect(maps.Keys(legacy))
	assert.Equal(t, 3, s.Len())
	assert.Assert(t, s.Contains(2))
}

// TestGenericsCoverage exercises representative element types: a struct key
// and uuid.UUID, in addition to the string/int coverage above.
func TestGenericsCoverage(t *testing.T) {
	type key struct {
		A string
		B int
	}
	ks := Of(key{"x", 1}, key{"y", 2})
	assert.Assert(t, ks.Contains(key{"x", 1}))
	assert.Assert(t, !ks.Contains(key{"x", 2}))

	u1, u2 := uuid.New(), uuid.New()
	us := Of(u1)
	assert.Assert(t, us.Contains(u1))
	assert.Assert(t, !us.Contains(u2))
}
