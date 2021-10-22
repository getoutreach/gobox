// Package async has helper utilities for running async code with
// proper tracing.
//
// When starting go routines, use async.Run:
//
//     // this starts a go routine
//     async.Run(ctx, async.Func(func(ctx context.Context) error {
//        ctx = trace.StartCall(ctx, "...")
//        defer trace.EndCall(ctx)
//        .... do whatever needs to be done ...
//        .... just return error to abort loop ....
//     }))
//
// If the long running activity involves fetching from a reader or
// some other iterative pattern, use async.Loop
//
//    // this starts a go routine
//    async.Loop(ctx, async.Func(func(ctx context.Context) error {
//        ctx = trace.StartCall(ctx, "...")
//        defer trace.EndCall(ctx)
//       .... do whatever... return errors to abort loop
//       ... maybe async.Sleep(ctx, time.Minute)
//    }))
//
// Note:  async.Loop terminates the loop when the inner function
// returns an error or the context is canceled.
//
// To wait for all go-routines to terminate, use Tasks.Wait.  See
// examples for using Tasks
package async

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/trace"
)

// Runner is the default interface for a runner function
type Runner interface {
	Run(ctx context.Context) error
}

// Tasks runs tasks
type Tasks struct {
	Name string
	sync.WaitGroup
}

// NewTasks creates new instance of Tasks
func NewTasks(name string) *Tasks {
	return &Tasks{Name: name}
}

// Run executes a single asynchronous task.
//
// It creates a new trace for the task and passes through deadlines.
func (t *Tasks) Run(ctx context.Context, r Runner) {
	t.WaitGroup.Add(1)
	go func() {
		defer t.WaitGroup.Done()
		ctx2 := trace.StartTrace(ctx, t.Name)
		defer trace.End(ctx2)
		if err := r.Run(ctx2); err != nil && err != ctx2.Err() {
			log.Error(ctx2, t.Name, events.NewErrorInfo(err))
		}
	}()
}

// Loop repeatedly executes the provided task until it returns false
// or the context is canceled.
func (t *Tasks) Loop(ctx context.Context, r Runner) {
	run := func(ctx context.Context) bool {
		ctx2 := trace.StartTrace(ctx, t.Name)
		defer trace.End(ctx2)
		if err := r.Run(ctx2); err != nil && err != ctx2.Err() {
			log.Error(ctx2, t.Name, events.NewErrorInfo(err))
			return true
		}
		return false
	}

	t.WaitGroup.Add(1)
	go func() {
		defer t.WaitGroup.Done()
		for ctx.Err() == nil {
			if ok := run(ctx); ok {
				break
			}
		}
	}()
}

// Default is the default runner
var Default = NewTasks("async.run")

// Run executes a single asynchronous task.
//
// It creates a new trace for the task and passes through deadlines.
func Run(ctx context.Context, r Runner) {
	Default.Run(ctx, r)
}

// RunBackground executes a single asynchronous task with background context
//
// It creates a new trace for the task and passes through deadlines.
func RunBackground(ctx context.Context, r Runner) {
	Run(context.Background(), r)
}

// Loop repeatedly executes the provided task until it returns false
// or the context is canceled.
func Loop(ctx context.Context, r Runner) {
	Default.Loop(ctx, r)
}

// Func is a helper that implements the Runner interface
type Func func(ctx context.Context) error

// Run implements the Runner interface
func (f Func) Run(ctx context.Context) error {
	return f(ctx)
}

// Sleep sleeps for the specified duration but can be canceled if the
// context is canceled or the context has an earlier deadline/timeout.
func Sleep(ctx context.Context, duration time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	<-ctx.Done()
}

// SleepUntil sleeps until the specified deadline but can be canceled
// if the context is canceled or has an earlier deadline.
func SleepUntil(ctx context.Context, deadline time.Time) {
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	<-ctx.Done()
}

// MutexWithContext is a lock that supports context cancellation.
//
// Note that unlike the `sync.Locker` style of mutex, this one's `Lock` method
// can fail and you must check its return value.
type MutexWithContext struct {
	sem *semaphore.Weighted
}

// NewMutexWithContext creates a new MutexWithContext instance.
func NewMutexWithContext() *MutexWithContext {
	return &MutexWithContext{semaphore.NewWeighted(1)}
}

// Lock acquires the mutex, blocking if it is unavailable.
//
// Unlike `sync.Mutex.Lock()`, this function can fail.  It is responsibility of
// the caller to check the returned error and not proceed if it is non-nil.
func (m *MutexWithContext) Lock(ctx context.Context) error {
	return m.sem.Acquire(ctx, 1)
}

// Unlock releases the mutex, allowing the next waiter to proceed.
func (m *MutexWithContext) Unlock() {
	m.sem.Release(1)
}