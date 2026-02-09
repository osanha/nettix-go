package util

import (
	"log/slog"
)

// AbstractRunnable manages the lifecycle of a goroutine-based execution.
type AbstractRunnable struct {
	*AbstractStartable
	stopCh  chan struct{}
	doneCh  chan struct{}
	Running func() error
}

// NewAbstractRunnable creates a new AbstractRunnable with the given name.
func NewAbstractRunnable(name string) *AbstractRunnable {
	r := &AbstractRunnable{
		AbstractStartable: NewAbstractStartable(name),
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
	}

	r.AbstractStartable.SetUp = r.setUp
	r.AbstractStartable.TearDown = r.tearDown

	return r
}

func (r *AbstractRunnable) setUp() error {
	go r.run()
	return nil
}

func (r *AbstractRunnable) tearDown() error {
	close(r.stopCh)
	<-r.doneCh
	return nil
}

func (r *AbstractRunnable) run() {
	defer close(r.doneCh)

	for {
		select {
		case <-r.stopCh:
			return
		default:
			if r.Running != nil {
				if err := r.Running(); err != nil {
					r.Logger().Error("Runnable error", "error", err)
				}
			}
		}
	}
}

// StopChannel returns the stop channel for checking termination.
func (r *AbstractRunnable) StopChannel() <-chan struct{} {
	return r.stopCh
}

// IsRunning returns true if the runnable is still running.
func (r *AbstractRunnable) IsRunning() bool {
	select {
	case <-r.stopCh:
		return false
	default:
		return r.State() == StateRunning
	}
}

// Sleep sleeps for the given duration or until stop is requested.
// Returns true if sleep completed, false if interrupted by stop.
func (r *AbstractRunnable) Sleep(logger *slog.Logger, duration int64) bool {
	// Implementation using time.After would be here
	return true
}
