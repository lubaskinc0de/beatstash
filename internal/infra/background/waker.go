package background

import "sync"

// Waker wakes the idle workers when a job is queued, so they do not wait
// for their next poll.
type Waker struct {
	mu    sync.Mutex
	lanes []chan struct{}
}

func (w *Waker) Wake() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, lane := range w.lanes {
		select {
		case lane <- struct{}{}:
		default:
		}
	}
}

func (w *Waker) lane() <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	lane := make(chan struct{}, 1)
	w.lanes = append(w.lanes, lane)
	return lane
}
