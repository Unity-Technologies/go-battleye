package battleye

import (
	"sync"
)

// isDone returns true if the done channel has been closed.
//
// While safe, using this method can be racey so consider using Done instead.
func isDone(done chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// done represents a done channel.
type done struct {
	ch chan struct{}
	mu sync.RWMutex
}

// newDone returns a new Done channel.
func newDone() *done {
	return &done{ch: make(chan struct{})}
}

// IsDone returns true if Done() has been called called, false otherwise.
func (d *done) IsDone() bool {
	d.mu.RLock()
	// We don't use defer to avoid the overhead.
	done := isDone(d.ch)
	d.mu.RUnlock()

	return done
}

// C returns the done channel for reading.
func (d *done) C() <-chan struct{} {
	return d.ch
}

// Done marks Done as done.
func (d *done) Done() {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.ch:
		return
	default:
		close(d.ch)
	}
}
