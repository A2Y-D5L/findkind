package util

// Semaphore is a tiny non-blocking semaphore that prevents WalkDir dead-lock.
type Semaphore struct{ ch chan struct{} }

func NewSemaphore(max int) *Semaphore { return &Semaphore{ch: make(chan struct{}, max)} }

// TryAcquire returns true if a slot was obtained without blocking.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release frees one slot.
func (s *Semaphore) Release() { <-s.ch }
