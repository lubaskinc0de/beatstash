// Package runs lets a caller wake a background loop and wait for its runs.
package runs

import (
	"context"
	"sync"
)

// Runs marks the runs of one loop. A caller waits for a run that starts
// after it asks: one already going may have missed what the caller did.
type Runs struct {
	mu sync.Mutex
	// next closes when the next run to start is over.
	next chan struct{}
	wake chan struct{}
}

// Start marks the start of a run; the returned func marks its end.
func (r *Runs) Start() (end func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	done := r.nextLocked()
	r.next = nil
	return func() { close(done) }
}

// Wait blocks until a run that starts after the call is over.
func (r *Runs) Wait(ctx context.Context) error {
	r.mu.Lock()
	done := r.nextLocked()
	r.mu.Unlock()
	return wait(ctx, done)
}

// Now wakes the loop and waits for the run that follows.
func (r *Runs) Now(ctx context.Context) error {
	r.mu.Lock()
	done := r.nextLocked()
	r.mu.Unlock()
	r.Wake()
	return wait(ctx, done)
}

// Wake makes the loop run now instead of at its next tick.
func (r *Runs) Wake() {
	select {
	case r.wakeChan() <- struct{}{}:
	default:
	}
}

// Woken is what the loop listens to besides its ticker.
func (r *Runs) Woken() <-chan struct{} {
	return r.wakeChan()
}

func (r *Runs) wakeChan() chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.wake == nil {
		r.wake = make(chan struct{}, 1)
	}
	return r.wake
}

func (r *Runs) nextLocked() chan struct{} {
	if r.next == nil {
		r.next = make(chan struct{})
	}
	return r.next
}

func wait(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
