package util

import (
	"os"
	"os/signal"
	"syscall"
)

// AbstractStoppable registers a shutdown hook that runs automatically
// when the application terminates.
type AbstractStoppable struct {
	stopFunc func()
	done     chan struct{}
}

// NewAbstractStoppable creates a new AbstractStoppable with the given stop function.
func NewAbstractStoppable(stopFunc func()) *AbstractStoppable {
	s := &AbstractStoppable{
		stopFunc: stopFunc,
		done:     make(chan struct{}),
	}
	s.registerShutdownHook()
	return s
}

func (s *AbstractStoppable) registerShutdownHook() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		if s.stopFunc != nil {
			s.stopFunc()
		}
		close(s.done)
	}()
}

// Wait blocks until the shutdown signal is received.
func (s *AbstractStoppable) Wait() {
	<-s.done
}
