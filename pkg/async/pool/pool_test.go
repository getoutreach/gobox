package pool_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"runtime/pprof"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/async/pool"
	"github.com/getoutreach/gobox/pkg/orerr"
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
	Items       int
	Size        pool.OptionFunc
	ResizeEvery time.Duration
	Expected    []string
	StartedAt   time.Time
	Results     stringChan
	Pool        *pool.Pool
	Context     context.Context
	Cancel      context.CancelFunc
}

func TestHasCorrectOutput(t *testing.T) {
	s := runPool(context.Background(), &testState{Items: 10, Size: pool.ConstantSize(10)})
	defer s.Pool.Close()
	defer s.Cancel()
	assert.Assert(t, WithinDuration(time.Now(), s.StartedAt, 100*time.Millisecond))
	actual := s.Results.ToSlice()
	sort.Strings(s.Expected)
	sort.Strings(actual)
	assert.DeepEqual(t, s.Expected, actual)
}

func TestWeCantEnqueueWhenStopped(t *testing.T) {
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

func TestGracefullyStops(t *testing.T) {
	size := 10
	s := runPool(context.Background(), &testState{Items: 10, Size: pool.ConstantSize(size)})
	defer s.Cancel()

	// When pool was running there were pool goroutines
	assert.Assert(t, waitForWorkers(t, size), "workers not detected")

	s.Pool.Close()

	assert.Assert(t, waitForWorkers(t, 0), "workers detected")
}

// TestPoolGrows checks number of running goroutines can't be execute using shuffler that run tests in parallel
func TestPoolGrows(t *testing.T) {
	var size = make(chan int, 1)

	resize := func(s int) {
		fmt.Println("resizing to:", s)
		size <- s
	}

	savedSize := 1
	s := &testState{
		Items: 10,
		Size: pool.Size(func() int {
			select {
			case s := <-size:
				fmt.Println("size saved:", s)
				savedSize = s
				return s
			default:
				return savedSize
			}
		}),
		ResizeEvery: 1 * time.Millisecond,
	}
	runPool(context.Background(), s)

	defer s.Cancel()
	defer s.Pool.Close()

	assert.Assert(t, waitForWorkers(t, 1), "workers not detected")

	resize(10)
	assert.Assert(t, waitForWorkers(t, 10), "workers not detected")

	resize(2)
	assert.Assert(t, waitForWorkers(t, 2), "workers not detected")
}

func numWorkers() int {
	buf := bytes.Buffer{}
	b := bufio.NewWriter(&buf)
	profile := pprof.Lookup("goroutine")
	profile.WriteTo(b, 2)
	b.Flush()
	matches := regexp.MustCompile(`\(\*Pool\).worker\(`).FindAllString(buf.String(), -1)
	return len(matches)
}

func waitForWorkers(t *testing.T, num int) bool {
	current := 0
	for i := 0; i < 20; i++ {
		current = numWorkers()
		if current == num {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("workers are %v not %v", current, num)
	return false
}

// Test that an empty pool with a zero-length buffer rejects all tasks.
func TestPoolWithZeroBuffer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	jobs := 10

	p := pool.New(
		ctx, pool.BufferLength(0), pool.ConstantSize(0), pool.RejectWhenFull,
	)

	var wg sync.WaitGroup
	var passed, cancelled, rejected int32

	wg.Add(jobs)
	// schedule 10 jobs, only 5 should pass
	for i := 0; i < jobs; i++ {
		p.Schedule(ctx, async.Func(func(ctx context.Context) error {
			defer wg.Done()
			if err := ctx.Err(); err != nil { // return immediately if ctx is closed
				var lee orerr.LimitExceededError
				if errors.As(err, &lee) {
					atomic.AddInt32(&rejected, 1)
					return nil
				}

				atomic.AddInt32(&cancelled, 1)
				return nil
			}

			<-ctx.Done()
			atomic.AddInt32(&passed, 1)
			return nil
		}))
	}

	go cancel()
	wg.Wait() // wait till all jobs are processed/rejected

	assert.Equal(t, 0, int(atomic.LoadInt32(&passed)), "should process 0 jobs", 0)
	assert.Equal(t, jobs, int(atomic.LoadInt32(&rejected)), "should reject %d jobs", jobs)
	assert.Equal(t, 0, int(atomic.LoadInt32(&cancelled)), "should cancel 0 jobs", 0)
}

func runPool(ctx context.Context, s *testState) *testState {
	s.Results = make(stringChan, s.Items)
	ctx, cancel := context.WithTimeout(ctx, 5000*time.Millisecond)

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
				log.Println(">  Item written", i, "err:", ctx.Err())
				return nil
			}))
		}(i)
	}

	log.Println("> waiting on pool")

	wg.Wait()

	log.Println("> pool done")

	close(s.Results)
	return s
}
