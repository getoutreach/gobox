// Copyright 2024 Outreach Corporation. All Rights Reserved.
//
// Description: original sync.Once tests

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package async_test

import (
	. "github.com/getoutreach/gobox/pkg/async"

	"testing"
)

type one int

func (o *one) Increment() {
	*o++
}

func run(t *testing.T, once *RetryableOnce, o *one, c chan bool) {
	once.Do(func() bool {
		o.Increment()
		return true
	})
	if v := *o; v != 1 {
		t.Errorf("once failed inside run: %d is not 1", v)
	}
	c <- true
}

func TestOnce(t *testing.T) {
	o := new(one)
	once := new(RetryableOnce)
	c := make(chan bool)
	const N = 10
	for i := 0; i < N; i++ {
		go run(t, once, o, c)
	}
	for i := 0; i < N; i++ {
		<-c
	}
	if *o != 1 {
		t.Errorf("once failed outside run: %d is not 1", *o)
	}
}

func TestOncePanic(t *testing.T) {
	var once RetryableOnce
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Once.Do did not panic")
			}
		}()
		once.Do(func() bool {
			panic("failed")
		})
	}()

	// panic can't be retried. This verifies that `once` is done after a panic
	once.Do(func() bool {
		t.Fatalf("Once.Do called twice")
		return true
	})
}

func BenchmarkOnce(b *testing.B) {
	var once RetryableOnce
	f := func() bool { return true }
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			once.Do(f)
		}
	})
}
