# async/pool

This package is currently deprecated in favor of
[github.com/sourcegraph/conc/pool](https://pkg.go.dev/github.com/sourcegraph/conc/pool).

## Migrating

Most of the functionality from the original package is available in the
`conc/pool` package. The main difference is that there is no ability to
control what happens when a worker is unavailable. Previously, one could
include a dynamically resizable buffer or constant size buffer and how
Schedule would react when the buffer was full. This is no longer
possible, instead there is no buffer and the `Go` method will always
block until a worker is available.

### Creating a constant pool for workers (`pool.New`)

The majority use case of this package was to create a pool of
goroutines, of a specific size, that would process work.

#### Old

```go
p := pool.New(ctx,
  pool.ConstantSize(numOfWorkers),
)

p.Schedule(ctx, async.Func(func(ctx context.Context) error {
  // Do work
}))
```

#### New

```go
p := pool.New().WithContext(ctx).WithMaxGoroutines(numOfWorkers)

// blocks if there is no available worker.
p.Go(func(ctx context.Context) error {
  // Do work
})

```

#### Creating a Pool and waiting for it to finish (`pool.WithWait`)

The `pool.WithWait` function is no longer necessary. Instead, the
`pool.Pool` type has a `Wait` method that will block until all workers
have finished.

#### Old

```go
p := pool.New(ctx,
  pool.ConstantSize(numOfWorkers),
)

var wait func()
p, wait = pool.WithWait(p)

go p.Schedule(ctx, async.Func(func(ctx context.Context) error {
  // Do work
}))

wait()
```

#### New

```go
p := pool.New().WithContext(ctx).WithMaxGoroutines(numOfWorkers)

go p.Go(func(ctx context.Context) error {
  // Do work
})

p.Wait()
```

## Original README

See [ORIGINAL_README.md](ORIGINAL_README.md) for the original README.
