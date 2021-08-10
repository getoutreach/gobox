package pool_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"testing"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/async/pool"
)

//nolint:unparam
func benchmarkWorkerN(b *testing.B, size, items int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := pool.New(ctx, pool.ConstantSize(size))

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		wg := new(sync.WaitGroup)
		results := make(stringChan, items)
		for i := 0; i < items; i++ {
			wg.Add(1)
			// Don't move this line into closure, we want to test that data will be correct
			func(i int) {
				p.Schedule(ctx, async.Func(func(ctx context.Context) error {
					var sum [32]byte
					for z := 0; z < 1000; z++ {
						sum = sha256.Sum256([]byte(fmt.Sprintf("task_%d", i)))
					}
					results <- string(sum[:])
					wg.Done()
					return nil
				}))
			}(i)
		}
		wg.Wait()
		close(results)
	}
}

func benchmarkGoN(b *testing.B, items int) {
	for n := 0; n < b.N; n++ {
		wg := new(sync.WaitGroup)
		results := make(stringChan, items)
		for i := 0; i < items; i++ {
			wg.Add(1)
			// Don't move this line into closure, we want to test that data will be correct
			go func(i int) {
				var sum [32]byte
				for z := 0; z < 1000; z++ {
					sum = sha256.Sum256([]byte(fmt.Sprintf("task_%d", i)))
				}
				results <- string(sum[:])
				wg.Done()
			}(i)
		}
		wg.Wait()
		close(results)
	}
}

func BenchmarkPool1000(b *testing.B) { benchmarkWorkerN(b, 1000, 10000) }
func BenchmarkPool100(b *testing.B)  { benchmarkWorkerN(b, 100, 10000) }
func BenchmarkPool10(b *testing.B)   { benchmarkWorkerN(b, 10, 10000) }
func BenchmarkPool5(b *testing.B)    { benchmarkWorkerN(b, 5, 10000) }

func BenchmarkPureGo(b *testing.B) { benchmarkGoN(b, 10000) }
