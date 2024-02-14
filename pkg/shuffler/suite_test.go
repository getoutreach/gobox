package shuffler

import (
	"reflect"
	"testing"
)

type testingTestSuite struct{}

func (ts *testingTestSuite) One()                   {}
func (ts *testingTestSuite) Two()                   {}
func (ts *testingTestSuite) TestOne(_ *testing.T)   {}
func (ts *testingTestSuite) TestTwo(_ *testing.T)   {}
func (ts *testingTestSuite) TestThree(_ *testing.T) {}

func TestEverythingWorks(_ *testing.T) {}

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

func TestThatTestsAreShuffledDeterministically(t *testing.T) {
	one := newInternalTestSetup()
	two := newInternalTestSetup()

	length := len(two)

	// check that the two slices are identical to start (DeepEqual will
	// ensure the elements are in the same order
	if !reflect.DeepEqual(one, two) {
		t.Fail()
	}

	// Set a specific seed to create deterministic order
	seed := int64(3)
	shuffleSeed = &seed

	two = shuffleTests(two, t)

	if len(two) != length {
		t.Fail()
	}

	// And now use DeepEqual to check that the order of the test slices are
	// now distinct
	if reflect.DeepEqual(one, two) {
		t.Fail()
	}

	// Verify the specific order of the shuffled tests are always the same
	testOrder := []testing.InternalTest{
		{Name: "d"},
		{Name: "c"},
		{Name: "b"},
		{Name: "a"},
		{Name: "g"},
		{Name: "e"},
		{Name: "e"},
		{Name: "e"},
		{Name: "f"},
	}
	if !reflect.DeepEqual(two, testOrder) {
		t.Fail()
	}
}
