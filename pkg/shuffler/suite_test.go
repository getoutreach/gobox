package shuffler

import (
	"reflect"
	"testing"
)

type testingTestSuite struct{}

func (ts *testingTestSuite) One()                   {}
func (ts *testingTestSuite) Two()                   {}
func (ts *testingTestSuite) TestOne(t *testing.T)   {}
func (ts *testingTestSuite) TestTwo(t *testing.T)   {}
func (ts *testingTestSuite) TestThree(t *testing.T) {}

func TestEverythingWorks(t *testing.T) {}

func TestOnlyTestMethodsResolved(t *testing.T) {
	ts := new(testingTestSuite)

	resolved := resolveTests(ts)

	// This should resolve 3 methods (TestOne, TestTwo, and TestThree)
	if len(resolved) != 3 {
		t.Fail()
	}

	// The resolved method names should only be TestOne, TestTwo, or TestThree
	for _, test := range resolved {
		if !(test.Name == "TestOne" || test.Name == "TestTwo" || test.Name == "TestThree") {
			t.Fail()
		}
	}
}

func newInternalTestSetup() []testing.InternalTest {
	return []testing.InternalTest{
		{
			Name: "a",
		},
		{
			Name: "b",
		},
		{
			Name: "c",
		},
		{
			Name: "d",
		},
		{
			Name: "e",
		},
		{
			Name: "e",
		},
		{
			Name: "f",
		},
		{
			Name: "g",
		},
		{
			Name: "e",
		},
	}
}

func TestThatTestsAreShuffled(t *testing.T) {
	one := newInternalTestSetup()
	two := newInternalTestSetup()

	length := len(two)

	// check that the two slices are identical to start (DeepEqual will
	// ensure the elements are in the same order
	if !reflect.DeepEqual(one, two) {
		t.Fail()
	}

	two = shuffleTests(two, t)

	if len(two) != length {
		t.Fail()
	}

	// And now use DeepEqual to check that the order of the test slices are
	// now distinct
	if reflect.DeepEqual(one, two) {
		t.Fail()
	}
}
