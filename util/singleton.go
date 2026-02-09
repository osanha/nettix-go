package util

import (
	"runtime"
	"sync"
	"time"
)

// Singleton provides shared resources for the nettix framework.
var Singleton = &singleton{
	workerCount: int(float64(runtime.NumCPU()) * 2.5),
}

type singleton struct {
	workerCount int
	timerOnce   sync.Once
	timer       *Timer
}

// Timer returns the shared timer instance.
func (s *singleton) Timer() *Timer {
	s.timerOnce.Do(func() {
		s.timer = NewTimer()
	})
	return s.timer
}

// WorkerCount returns the default number of worker goroutines.
func (s *singleton) WorkerCount() int {
	return s.workerCount
}

// SetWorkerCount sets the default number of worker goroutines.
func (s *singleton) SetWorkerCount(count int) {
	s.workerCount = count
}

// Timer provides a hashed wheel timer implementation for scheduling tasks.
type Timer struct {
	stopCh chan struct{}
}

// NewTimer creates a new Timer.
func NewTimer() *Timer {
	t := &Timer{
		stopCh: make(chan struct{}),
	}
	return t
}

// Timeout represents a scheduled timeout task.
type Timeout struct {
	timer    *time.Timer
	task     func()
	cancelled bool
	mu       sync.Mutex
}

// NewTimeout schedules a task to run after the specified delay.
func (t *Timer) NewTimeout(task func(), delay time.Duration) *Timeout {
	timeout := &Timeout{
		task: task,
	}

	timeout.timer = time.AfterFunc(delay, func() {
		timeout.mu.Lock()
		if !timeout.cancelled {
			timeout.mu.Unlock()
			task()
		} else {
			timeout.mu.Unlock()
		}
	})

	return timeout
}

// Cancel cancels the timeout.
func (t *Timeout) Cancel() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancelled {
		return false
	}

	t.cancelled = true
	return t.timer.Stop()
}

// IsCancelled returns true if the timeout was cancelled.
func (t *Timeout) IsCancelled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancelled
}

// Stop stops the timer.
func (t *Timer) Stop() {
	close(t.stopCh)
}
