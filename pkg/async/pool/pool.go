package pool

import (
	"context"
	"sync"
	"time"

	"github.com/getoutreach/gobox/pkg/events"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
)

// Option allows to functional options pattern to configure pool
type Option interface {
	Apply(*Options)
}

// OptionFunc help to implement Option interface
type OptionFunc func(*Options)

// Apply implementation of Option interface
func (of OptionFunc) Apply(opts *Options) {
	of(opts)
}

// SizeFunc tells the pool whether it should increase or decrease number of workers
type SizeFunc func() int

// ConstantSize provides
func ConstantSize(size int) OptionFunc {
	return func(opts *Options) {
		opts.Size = func() int {
			return size
		}
	}
}

// Size helps to set Size option
func Size(sizeFunc SizeFunc) OptionFunc {
	return func(opts *Options) {
		opts.Size = sizeFunc
	}
}

// BufferLength helps to set BufferLength option
func BufferLength(size int) OptionFunc {
	return func(opts *Options) {
		opts.BufferLength = size
	}
}

// ResizeEvery helps to set ResizeEvery option
func ResizeEvery(d time.Duration) OptionFunc {
	return func(opts *Options) {
		opts.ResizeEvery = d
	}
}

// ScheduleBehavior defines the behavior of pool Schedule method
type ScheduleBehavior func(context.Context, chan unit, async.Runner) error

// Apply implementation of Option interface
func (sb ScheduleBehavior) Apply(opts *Options) {
	opts.ScheduleBehavior = sb
}

// RejectWhenFull tries to schedule async.Runner for period when context is alive
// When underlying buffered channel is full then it cancels the context with orerr.LimitExceededError
var RejectWhenFull = ScheduleBehavior(func(ctx context.Context, queue chan unit, r async.Runner) error {
	ctx, cancel := orerr.CancelWithError(ctx)
	select {
	case <-ctx.Done():
		return r.Run(ctx)
	case queue <- unit{Context: ctx, Runner: r}:
		return nil
	default:
		cancel(orerr.LimitExceededError{
			Kind: "PoolQueue",
		})
		return r.Run(ctx)
	}
})

// WaitWhenFull tries to schedule async.Runner for period when context is alive
// It blocks When underlying buffered channel is full
var WaitWhenFull = ScheduleBehavior(func(ctx context.Context, queue chan unit, r async.Runner) error {
	select {
	case <-ctx.Done():
		return r.Run(ctx)
	case queue <- unit{Context: ctx, Runner: r}:
		return nil
	}
})

// A Options provides pool configuration
type Options struct {
	// Size allows to dynamically resolve number of workers that should spawned
	Size SizeFunc

	// ResizeEvery defined intervals when pool will be resized (shrank or grown)
	ResizeEvery time.Duration

	// ScheduleBehavior defines how exactly will Schedule method behave.
	// The WaitWhenFull is used by default if no value is provided
	ScheduleBehavior ScheduleBehavior

	// BufferLength defines size of buffered channel queue
	BufferLength int

	// Pool name for logging reasons
	Name string
}

// Pool structure
type Pool struct {
	// Protects the context during cancelation
	cancel  func(error)
	context context.Context
	closed  chan struct{}
	opts    *Options
	queue   chan unit
	wg      *sync.WaitGroup
}

// New creates new instance of Pool and start goroutine that will spawn the workers
// Call Close() to release pool resource
func New(ctx context.Context, options ...Option) *Pool {
	// default values
	var opts = &Options{
		ScheduleBehavior: WaitWhenFull,
		ResizeEvery:      1 * time.Minute,
		Name:             "default",
		BufferLength:     10 * 1000,
	}
	ConstantSize(1000).Apply(opts)

	for _, o := range options {
		o.Apply(opts)
	}

	ctx, cancel := orerr.CancelWithError(ctx)
	p := &Pool{
		wg:      new(sync.WaitGroup),
		queue:   make(chan unit, opts.BufferLength),
		opts:    opts,
		cancel:  cancel,
		context: ctx,
		closed:  make(chan struct{}),
	}
	// spawn initial workers synchronously
	cancellations := p.spawnWorkers(ctx, p.opts.Size())
	p.wg.Add(1)
	go p.run(ctx, cancellations)
	return p
}

func (p *Pool) spawnWorkers(ctx context.Context, n int) cancellations {
	cancels := make(cancellations, 0, n)
	for i := 0; i < n; i++ {
		workerCtx, cancel := context.WithCancel(ctx)
		cancels = append(cancels, cancel)
		p.wg.Add(1)
		go p.worker(workerCtx)
	}

	return cancels
}

func (p *Pool) run(ctx context.Context, cancellations cancellations) {
	defer p.wg.Done()
	var (
		prevSize, delta int
	)
	for ctx.Err() == nil {
		prevSize = len(cancellations)
		delta = p.opts.Size() - prevSize
		if delta < 0 {
			// Cancel some workers
			cancellations = cancellations.Shrink(-delta)
		} else if delta > 0 {
			// Spawn new workers
			cancellations = append(cancellations, p.spawnWorkers(ctx, delta)...)
		}
		if delta != 0 {
			log.Info(ctx, "async.pool resized",
				log.F{
					"pool":     p.opts.Name,
					"size":     len(cancellations),
					"previous": prevSize,
				},
			)
		}

		select {
		case <-time.After(p.opts.ResizeEvery):
			continue
		case <-p.closed:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	var u unit
	for {
		select {
		case u = <-p.queue:
			//nolint:errcheck
			_ = u.Runner.Run(u.Context)
		case <-p.closed:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Close blocks until all workers finshes current items and terminates
func (p *Pool) Close() {
	p.cancel(&orerr.ShutdownError{Err: context.Canceled})
	close(p.closed)
	p.wg.Wait()
}

// Schedule tries to schedule runner for processing in the pool
// It is required to check provided context for an error.
// The async.Runner interface will be called in all cases:
// - When item gets successfully scheduled and withdrawn by worker
// - When the given context is Done and item is not scheduled (Timeout, buffered queue full)
// - When pool is in shutdown phase.
func (p *Pool) Schedule(ctx context.Context, r async.Runner) error {
	// Check whether pool is alive
	if p.context.Err() != nil {
		ctxErr, cancel := orerr.CancelWithError(ctx)
		cancel(p.context.Err())
		return r.Run(ctxErr)
	}
	return p.opts.ScheduleBehavior(ctx, p.queue, r)
}

type cancellations []context.CancelFunc

// Shrink reduces size of slice and calls context.CancelFunc on those that will be removed
func (c cancellations) Shrink(by int) cancellations {
	if by == 0 {
		return c
	}
	l := len(c)
	for i := l - by; i < l; i++ {
		c[i]()
	}
	if by >= l {
		return c[:0]
	}
	c = c[:(len(c) - by)]
	return c
}

type unit struct {
	Context context.Context
	Runner  async.Runner
}

type loggingScheduler struct {
	Inner Scheduler
	Name  string
}

func (w *loggingScheduler) Schedule(ctx context.Context, r async.Runner) error {
	return w.log(ctx, w.Inner.Schedule(ctx, async.Func(func(ctx context.Context) error {
		return w.log(ctx, r.Run(ctx))
	})))
}

// WithLogging creates a scheduler which logs the errors returned from the scheduling as well as executing phase
func WithLogging(name string, s Scheduler) Scheduler {
	return &loggingScheduler{Name: name, Inner: s}
}

func (w *loggingScheduler) log(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	log.Error(ctx, "async.pool runner error", log.F{"pool": w.Name}, events.NewErrorInfo(err))
	return err
}
