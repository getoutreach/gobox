package pool_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"runtime/pprof"
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
	assert.Assert(t, WithinDuration(time.Now(), s.StartedAt, 100*time.Millisecond))
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

	// When pool was running there were pool goroutines
	assert.Assert(t, InDelta(float64(s.NumGoroutineWithWorkers),
		float64(runtime.NumGoroutine()), float64(size+1)), "Num of Goroutine is higher then expected")
	s.Pool.Close()

	// We don't want to wait for the context to close the pool.
	assert.Assert(t, WithinDuration(time.Now(), s.StartedAt, 50*time.Millisecond))

	time.Sleep(5 * time.Millisecond)
	// After close all workers goroutines are dead
	assert.Assert(t, InDelta(float64(s.NumGoroutineOnStart),
		float64(runtime.NumGoroutine()), 1), "Num of Goroutine is higher then expected")
}

// TestPoolGrows checks number of running goroutines can't be execute using shuffler that run tests in parallel
func (suite) TestPoolGrows(t *testing.T) {
	var size = make(chan int, 1)
	//ng := 0

	resize := func(s int) {
		size <- s
	}

	savedSize := 1
	s := &testState{
		Items: 10,
		Size: pool.Size(func() int {
			select {
			case s := <-size:
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

	resize(10)
	waitForWorkers(t, 10)

	resize(2)
	waitForWorkers(t, 2)
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

func waitForWorkers(t *testing.T, num int) {
	current := 0
	for i := 0; i < 5; i++ {
		current = numWorkers()
		if current == num {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("workers are %v not %v", current, num)
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
