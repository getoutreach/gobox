package async_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/shuffler"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

type runWithCloser struct {
	isclosed bool
}

func (r *runWithCloser) Run(c context.Context) error {
	for {
		time.Sleep(time.Second)
		if c.Err() != nil {
			return c.Err()
		}
	}
}
func (r *runWithCloser) Close(c context.Context) error {
	r.isclosed = true
	return nil
}

func (suite) TestRunGroupErrorPropagation(t *testing.T) {
	ctx := context.Background()
	r1 := async.Func(func(c context.Context) error {
		return fmt.Errorf("oh no")
	})
	r2 := runWithCloser{}
	aggr := async.RunGroup([]async.Runner{&r1, &r2})
	err := aggr.Run(ctx)
	assert.ErrorContains(t, err, "oh no")
	assert.Equal(t, r2.isclosed, true, "Closed the infinite loop correctly")
}

func (suite) TestRunCancelPropagation(t *testing.T) {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	ctx, cancel := context.WithCancel(context.Background())
	async.Run(ctx, async.Func(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}))
	cancel()
	async.Default.Wait()
}

func (suite) TestRunDeadlinePropagation(t *testing.T) {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	async.Run(ctx, async.Func(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("no deadline!")
		}
		return nil
	}))

	cancel()
	async.Default.Wait()
}

func (suite) TestSleepUntil(t *testing.T) {
	now := time.Now()
	async.SleepUntil(context.Background(), time.Now().Add(-time.Second))
	assert.Assert(t, time.Since(now) <= time.Millisecond, "slept too long")

	async.SleepUntil(context.Background(), time.Now().Add(time.Millisecond))
	assert.Assert(t, time.Since(now) <= 5*time.Millisecond, "slept too long")
}

func (suite) TestRunTraceHeaders(t *testing.T) {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	async.Run(context.Background(), async.Func(func(ctx context.Context) error {
		if headers := trace.ToHeaders(ctx); len(headers) == 0 {
			t.Fatal("missing trace headers")
		}
		return nil
	}))
	async.Default.Wait()
}

func (suite) TestMutexWithContext_EarlyCancel(t *testing.T) {
	mutex := async.NewMutexWithContext()
	ctx1 := context.Background()

	err := mutex.Lock(ctx1)
	assert.NilError(t, err, "first lock acquisition should not fail")
	t.Cleanup(mutex.Unlock)

	ctx2 := context.Background()
	ctx2, cancel2 := context.WithCancel(ctx2)

	// Cancel right away, before second wait starts.
	cancel2()

	err = mutex.Lock(ctx2)
	assert.Assert(t, is.ErrorContains(err, ""), "expected error from cancellation")
}

func (suite) TestMutexWithContext_LateCancel(t *testing.T) {
	mutex := async.NewMutexWithContext()
	ctx1 := context.Background()

	err := mutex.Lock(ctx1)
	assert.NilError(t, err, "first lock acquisition should not fail")
	t.Cleanup(mutex.Unlock)

	ctx2 := context.Background()
	ctx2, cancel2 := context.WithTimeout(ctx2, time.Millisecond)
	t.Cleanup(cancel2)

	// We expect this will block until the timeout is reached.  There is
	// technically a race condition where the timeout happens before we have
	// a chance to block, in which case this test becomes equivalent to
	// `EarlyCancel`.  We can't prove that this never happens, but with a
	// long enough timeout it should be rare.
	err2 := mutex.Lock(ctx2)

	// Make sure the lock attempt was not successful.
	assert.Assert(t, is.ErrorContains(err2, ""), "expected error from cancellation")
}

func (suite) TestMutexWithContext_ExtraUnlock(t *testing.T) {
	mutex := async.NewMutexWithContext()
	ctx1 := context.Background()

	err := mutex.Lock(ctx1)
	assert.NilError(t, err, "first lock acquisition should not fail")

	mutex.Unlock()

	assert.Assert(t, is.Panics(func() { mutex.Unlock() }))
}

func ExampleTasks_run() {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasks := async.Tasks{Name: "example"}
	tasks.Run(ctx, async.Func(func(ctx context.Context) error {
		ctx = trace.StartCall(ctx, "example")
		defer trace.EndCall(ctx)

		fmt.Println("Run example")
		return nil
	}))

	cancel()
	tasks.Wait()

	// Output: Run example
}

func ExampleTasks_runBackground() {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	ctxMain, cancel := context.WithCancel(context.Background())

	async.RunBackground(ctxMain, async.Func(func(ctx context.Context) error {
		// the task is expected to run with background context
		// cancel the context passed into RunBackground() function should not
		// propagate to the context used by async.Func()
		cancel()
		ctx = trace.StartCall(ctx, "example")
		defer trace.EndCall(ctx)

		fmt.Println(ctx.Err())
		fmt.Println("Run example")
		return nil
	}))

	async.Default.Wait()

	// Output:
	// <nil>
	// Run example
}

func ExampleLoop() {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := 0
	async.Loop(ctx, async.Func(func(ctx context.Context) error {
		ctx = trace.StartCall(ctx, "example")
		defer trace.EndCall(ctx)

		if count < 3 {
			count++
			fmt.Println("count", count)
		} else {
			<-ctx.Done()
		}
		return nil
	}))

	time.Sleep(time.Millisecond * 5)
	cancel()
	async.Default.Wait()

	// Output:
	// count 1
	// count 2
	// count 3
}

func ExampleTasks_loop() {
	trlogs := tracetest.NewTraceLog("honeycomb")
	defer trlogs.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := 0
	tasks := async.Tasks{Name: "example"}
	tasks.Loop(ctx, async.Func(func(ctx context.Context) error {
		ctx = trace.StartCall(ctx, "example")
		defer trace.EndCall(ctx)

		if count < 3 {
			count++
			fmt.Println("count", count)
		} else {
			<-ctx.Done()
		}
		return nil
	}))

	async.Sleep(ctx, time.Millisecond*5)
	cancel()
	tasks.Wait()

	// Output:
	// count 1
	// count 2
	// count 3
}
