//go:build !or_e2e

// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides helpers for creating a shuffler test suite
package shuffler

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"runtime/debug"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/log"
)

// nolint:gochecknoglobals // Why: flag used in multiple places
var shuffleSeed = flag.Int64("shuffler.seed", 0, "Specify a seed for the randomization of test methods")

type TestSuite interface{}

// failOnPanic exists to ensure we capture the specific test context
// in the panic
func failOnPanic(t *testing.T, finished *bool) {
	err := recover()
	if !*finished && err == nil && !t.Failed() && !t.Skipped() {
		err = fmt.Errorf("panic(nil)")
	}
	if err != nil {
		t.Fatalf("test panicked: %v\n%s", err, debug.Stack())
	}
}

// Run takes test suites and runs all the exported Test* methods in
// random order.
func Run(t *testing.T, suites ...TestSuite) {
	var finished bool
	defer failOnPanic(t, &finished)

	tests := []testing.InternalTest{}
	for _, suite := range suites {
		tests = append(tests, resolveTests(suite)...)
	}
	tests = shuffleTests(tests, t)

	runTests(t, tests)
	finished = true
}

func shuffleTests(tests []testing.InternalTest, t *testing.T) []testing.InternalTest {
	var seed int64
	if *shuffleSeed == 0 {
		seed = time.Now().UnixNano()
	} else {
		seed = *shuffleSeed
	}
	t.Logf("Shuffling tests using seed %d", seed)
	rand.Seed(seed)

	rand.Shuffle(len(tests), func(i, j int) {
		tests[i], tests[j] = tests[j], tests[i]
	})

	return tests
}

// resolveTests uses the reflect package to build up the list of all the methods
// that our package consumers have defined on their artisanally crafted TestSuites
func resolveTests(suite TestSuite) []testing.InternalTest {
	tests := []testing.InternalTest{}

	finder := reflect.TypeOf(suite)
	re := regexp.MustCompile("^Test")

	for i := 0; i < finder.NumMethod(); i++ {
		method := finder.Method(i)
		if ok := re.MatchString(method.Name); !ok {
			continue
		}

		test := testing.InternalTest{
			Name: method.Name,
			F: func(t *testing.T) {
				var finished bool
				defer failOnPanic(t, &finished)

				method.Func.Call([]reflect.Value{
					reflect.ValueOf(suite),
					reflect.ValueOf(t),
				})
				finished = true
			},
		}
		tests = append(tests, test)
	}
	return tests
}

func runTests(t *testing.T, tests []testing.InternalTest) {
	if len(tests) == 0 {
		t.Log("No tests for this suite")
		return
	}

	for _, test := range tests {
		t.Run(test.Name, test.F)
		// Flush all debug logs from the test on failure
		if t.Failed() {
			log.Flush(context.TODO())
		} else {
			// Clear the debug queue so its contents don't contaminate the logs for the next test
			log.Purge(context.TODO())
		}
	}
}
