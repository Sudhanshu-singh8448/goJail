// Package worker provides a bounded concurrency pool with queuing.
package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// PoolStats holds runtime statistics about the worker pool.
type PoolStats struct {
	InFlight         int64
	Total            int64
	FailedInternal   int64
	LastInternalErr  time.Time
	HasInternalErr   bool
}

// Pool manages bounded concurrent execution of sandbox jobs.
// When all slots are taken, Submit blocks until a slot is available.
type Pool struct {
	sem             chan struct{}
	maxConcurrent   int
	inFlight        atomic.Int64
	total           atomic.Int64
	failedInternal  atomic.Int64
	mu              sync.Mutex
	lastInternalErr time.Time
	hasInternalErr  bool
}

// NewPool creates a new worker pool with the given concurrency limit.
func NewPool(maxConcurrent int) *Pool {
	return &Pool{
		sem:           make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// Submit queues a job for execution. It blocks until a slot is available
// or the context is cancelled. The job runs in a new goroutine.
func (p *Pool) Submit(ctx context.Context, fn func() error) error {
	// Wait for a slot or context cancellation
	select {
	case p.sem <- struct{}{}:
		// Got a slot
	case <-ctx.Done():
		return ctx.Err()
	}

	p.inFlight.Add(1)
	p.total.Add(1)

	// Run the job — the result channel lets the caller wait for completion
	go func() {
		defer func() {
			p.inFlight.Add(-1)
			<-p.sem
		}()

		if err := fn(); err != nil {
			p.failedInternal.Add(1)
			p.mu.Lock()
			p.lastInternalErr = time.Now()
			p.hasInternalErr = true
			p.mu.Unlock()
		}
	}()

	return nil
}

// SubmitAndWait queues a job and waits for it to complete.
// Returns the error from the job function.
func (p *Pool) SubmitAndWait(ctx context.Context, fn func() error) error {
	// Wait for a slot or context cancellation
	select {
	case p.sem <- struct{}{}:
		// Got a slot
	case <-ctx.Done():
		return ctx.Err()
	}

	p.inFlight.Add(1)
	p.total.Add(1)

	defer func() {
		p.inFlight.Add(-1)
		<-p.sem
	}()

	err := fn()
	if err != nil {
		p.failedInternal.Add(1)
		p.mu.Lock()
		p.lastInternalErr = time.Now()
		p.hasInternalErr = true
		p.mu.Unlock()
	}
	return err
}

// Stats returns the current pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return PoolStats{
		InFlight:       p.inFlight.Load(),
		Total:          p.total.Load(),
		FailedInternal: p.failedInternal.Load(),
		LastInternalErr: p.lastInternalErr,
		HasInternalErr:  p.hasInternalErr,
	}
}

// MaxConcurrent returns the pool's concurrency limit.
func (p *Pool) MaxConcurrent() int {
	return p.maxConcurrent
}
