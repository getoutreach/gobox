Goroutine worker pool structure

Universal structure for controlling the right level of concurrency. Excessive spawning of goroutines might lead to resource exhaustion or slowing down due to heavy context switching. 

You might think that Go routines are relatively cheap, but they are not for free. So choosing the right level of concurrency might help to improve overall performance and throughput of the system. See the benchmarks.

Something you might not realize when migrating from ruby. For example, every Http request in GO operates in goroutine already. There is no default limit on how many of these can be spawned (Possibly limit of opened descriptors, ports, etc). Under the DDOS or heavy request rate system will linearly slow down due to context switching. Ruby server in opposite usually runs with a fixed number of threads. If not a thread is available in the specified interval you are getting a timeout. 

Also, the pool prevents the application from being killed by OOM killer due to higher memory consumption

The library was developed for MailroomAPI and used to limit the number of concurrent calls to S3. When there was unbounded processing using just goroutines application got often killed by OOM killer under heavy load. 

https://github.com/getoutreach/mailroomapi/blob/master/internal/mailroomapi/storage/concurrent_message_reader.go

I have bumped into that issue already in the past. Here is a nice article to read. 
https://medium.com/smsjunk/handling-1-million-requests-per-minute-with-golang-f70ac505fcaa

# Benchmarks

 Goal here is to process 10000 "cpu heavy" operations. 

Benchmark | Description
-|-
BenchmarkPureGo | Spawning goroutine for each task and waiting for all of them to finish.
BenchmarkPool5-1000| Putting them into pool and processing just N items at once.


```
go test -benchmem -cpu 1,2,6 -run=^$ github.com/getoutreach/gobox/pkg/async/pool -v -bench '^Benchmark' 
goos: linux
goarch: amd64
pkg: github.com/getoutreach/gobox/pkg/async/pool
BenchmarkPool1000
BenchmarkPool1000-6            1        1560899231 ns/op        234332864 B/op  20031897 allocs/op
BenchmarkPool100
BenchmarkPool100-6             1        1156778012 ns/op        233000224 B/op  20020978 allocs/op
BenchmarkPool10
BenchmarkPool10-6              1        1092621750 ns/op        232935976 B/op  20020126 allocs/op
BenchmarkPool5
BenchmarkPool5-6               1        1313762356 ns/op        232909576 B/op  20019824 allocs/op
BenchmarkPureGo
BenchmarkPureGo-6              1        2264417948 ns/op        237819960 B/op  20034677 allocs/op
PASS
ok  	github.com/getoutreach/gobox/pkg/async/pool	28.360s

```

# Insides

- Currently structure utilize standard go channels that are more universal. 
- "Pool of workers" allow you to timeout without item being enqueued in opposite to pure "worker pool".

## Example
```go
package main

import (
	"fmt"
	"context"
	"fmt"
	"github.com/getoutreach/gobox/pkg/async/pool"
)

func main() {
	var (
		concurrency = 5
		items       = 10
		sleepFor    = 5 * time.Millisecond
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

    // Spawn pool of workers
	p := pool.New(ctx,
		pool.ConstantSize(concurrency),
		pool.ResizeEvery(5*time.Minute),
		pool.BufferLength(256),
		pool.WaitWhenFull,
	)
	defer p.Close()

    // Wrap it with timeout for schedule
	scheduler = pool.WithTimeout(5 * time.Millisecond, p)

    // Lets wait for items scheduled with this example
	scheduler, wait := pool.WithWait(scheduler)

	output := make(chan string, items)
	now := time.Now()

	for i := 0; i < items; i++ {
		func(i int) {
            // All input and output is captured by closure
			scheduler.Schedule(ctx, async.Func(func(ctx context.Context) error {
				if ctx.Err() != nil {
					// It is very important to check the context error:
					// - Given context might be done
					// - Underlying buffered channel is full
					// - Pool is in shutdown phase
					return ctx.Err()
				}
				time.Sleep(sleepFor)
				batchN := (time.Since(now) / (sleepFor))
				output <- fmt.Sprintf("task_%d_%d", batchN, i)
                // returned error is logged but not returned by Schedule function
				return nil
			})
		}(i)
	}
	wait()

	close(output)
	for s := range output{
		fmt.Println(s)
	}
	// Unordered output:
	// task_1_3
	// task_1_4
	// task_1_0
	// task_1_1
	// task_1_2
	// task_2_6
	// task_2_9
	// task_2_5
	// task_2_7
	// task_2_8
}
```
