// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provide capabilities to shuffle tests during a run.

// Package shuffler primarily provides the Suite struct that functions as a
// test runner and randomizer when embedded in your test struct. Methods
// defined on your suite struct will be resolved at runtime, and all tests
// that start with Test will be run. However, the key thing is that the test
// resolution will also randomize the order of the tests in the suite.
//
// The shuffler package also supports a command line flag to set the randomization
// seed. By default it uses time.Now().UnixNano(), but you can also set it by
// passing in `-shuffler.seed=<int>`. The seed used is always logged in the test
// output.
//
// Caveat:  The shuffler package only works for writing testing.T tests. If you
//
//	are writing benchmarks, please do not use this package. Also, it is
//	strongly recommended to not use this with t.Parallel().
//
// An example:
//
//	import (
//	    "github.com/gobox/pkg/shuffler"
//	    "github.com/stretchr/testify/assert"
//	)
//
//	// Define your suite struct, and embed it to get the predefined functionality
//	type YourTestSuite struct {
//	    shuffler.Suite
//	    Capacitors []Capacitor
//	}
//
//	// Any method that starts with Test will be run by the test suite, just
//	// like in the testing package
//	func (s *YourTestSuite) TestThatWeFluxCapacitors(t *testing.T) {
//	    f := NewFluxer()
//	    f.Flux(s.Capacitors)
//	    for c, _ := range s.Capacitors {
//	        assert.True(t, c.HasBeenFluxed)
//	    }
//	}
//
//	// Finally, you have to define one traditional test method to act as the
//	// equivalent of a TestMain(m *testing.M) function in the testing module
//	func TestCapacitorSuite(t *testing.T) {
//	    shuffler.Run(t, new(YourTestSuite))
//	}
//
// Deprecated: Test shuffling is supported of the box in Go 1.17 and later
// via the -shuffle flag. Use native go test functionality instead of this
// package.
package shuffler
