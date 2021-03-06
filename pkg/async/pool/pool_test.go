package pool_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/async/pool"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/shuffler"
	"gotest.tools/v3/assert"
)

type stringChan chan string

func (sr stringChan) ToSlice() []string {
	ss := []string{}
	for s := range sr {
		ss = append(ss, s)
	}
	return ss
}

type testState struct {
	Items                   int
	Size                    pool.OptionFunc
	ResizeEvery             time.Duration
	NumGoroutineOnStart     int
	NumGoroutineWithWorkers int
	Expected                []string
	StartedAt               time.Time
	Results                 stringChan
	Pool                    *pool.Pool
	Context                 context.Context
	Cancel                  context.CancelFunc
}

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestHasCorrectOutput(t *testing.T) {
	s := runPool(context.Background(), &testState{Items: 10, Size: pool.ConstantSize(10)})
	defer s.Pool.Close()
	defer s.Cancel()
	assert.Assert(t, WithinDuration(time.Now(), s.StartedAt, 8*time.Millisecond))
	actual := s.Results.ToSlice()
	sort.Strings(s.Expected)
	sort.Strings(actual)
	assert.DeepEqual(t, s.Expected, actual)
}

func (suite) TestWeCantEnqueueWhenStopped(t *testing.T) {
	s := runPool(context.Background(), &testState{Items: 10, Size: pool.ConstantSize(10)})
	defer s.Cancel()
	s.Pool.Close()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	processed := false
	err := s.Pool.Schedule(s.Context, async.Func(func(ctx context.Context) error {
		defer wg.Done()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		processed = true
		return nil
	}))
	wg.Wait()
	assert.Assert(t, processed == false, "Item got processed even when pool was closed") // NotEqual
	var shutdownErr *orerr.ShutdownError
	assert.Assert(t, errors.As(err, &shutdownErr))
}

func (suite) TestGracefullyStops(t *testing.T) {
	size := 10
	s := runPool(context.Background(), &testState{Items: 10, Size: pool.ConstantSize(size)})
	defer s.Cancel()
	defer s.Pool.Close()

	// When pool was running there were pool goroutines
	assert.Assert(t, InDelta(float64(s.NumGoroutineWithWorkers),
		float64(runtime.NumGoroutine()), float64(size+1)), "Num of Goroutine is higher then expected")
	s.Pool.Close()
	// After close all workers goroutines are dead
	assert.Assert(t, InDelta(float64(s.NumGoroutineOnStart),
		float64(runtime.NumGoroutine()), 1), "Num of Goroutine is higher then expected")
}

// TestPoolGrows checks number of running goroutines can't be execute using shuffler that run tests in parallel
func (suite) TestPoolGrows(t *testing.T) {
	var mu sync.Mutex
	var size = 1
	var resportResize = false
	wg := new(sync.WaitGroup)
	ng := 0

	waitForResize := func() {
		wg.Add(1)
		mu.Lock()
		resportResize = true
		mu.Unlock()
		wg.Wait()
		time.Sleep(5 * time.Millisecond)
		assert.Equal(t, size+1, runtime.NumGoroutine()-ng)
	}

	s := &testState{
		Items: 10,
		Size: pool.Size(func() int {
			mu.Lock()
			defer mu.Unlock()
			if resportResize {
				resportResize = false
				wg.Done()
			}
			return size
		}),
		ResizeEvery: 1 * time.Millisecond,
	}
	ng = runtime.NumGoroutine()

	runPool(context.Background(), s)

	defer s.Cancel()
	defer s.Pool.Close()

	waitForResize() // initital resize
	mu.Lock()
	size = 10
	mu.Unlock()
	waitForResize()
	mu.Lock()
	size = 2
	mu.Unlock()
	waitForResize()
}

func runPool(ctx context.Context, s *testState) *testState {
	s.NumGoroutineOnStart = runtime.NumGoroutine()
	s.Results = make(stringChan, s.Items)
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)

	if s.ResizeEvery == 0 {
		s.ResizeEvery = 5 * time.Millisecond
	}

	var (
		items = s.Items
		size  = s.Size

		p = pool.New(ctx,
			size,
			pool.ResizeEvery(s.ResizeEvery),
		)

		wg = new(sync.WaitGroup)
	)
	s.Pool = p
	s.StartedAt = time.Now()
	s.Cancel = cancel
	s.Context = ctx

	for i := 0; i < items; i++ {
		wg.Add(1)
		// Don't move this line into closure, we want to test that data will be correct
		s.Expected = append(s.Expected, fmt.Sprintf("task_%d", i))
		func(i int) {
			p.Schedule(ctx, async.Func(func(ctx context.Context) error {
				defer wg.Done()
				if ctx.Err() != nil {
					fmt.Println(ctx.Err())
					return ctx.Err()
				}
				time.Sleep(5 * time.Millisecond)
				s.Results <- fmt.Sprintf("task_%d", i)
				return nil
			}))
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.NumGoroutineWithWorkers = runtime.NumGoroutine()
	}()

	wg.Wait()
	close(s.Results)
	return s
}
