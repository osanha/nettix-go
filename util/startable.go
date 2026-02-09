package util

import (
	"log/slog"
	"sync"
)

// State represents the execution state of a startable component.
type State int

const (
	// StateReady indicates the component is ready to start.
	StateReady State = iota
	// StateRunning indicates the component is currently running.
	StateRunning
	// StateTerminated indicates the component has been terminated.
	StateTerminated
)

func (s State) String() string {
	switch s {
	case StateReady:
		return "READY"
	case StateRunning:
		return "RUNNING"
	case StateTerminated:
		return "TERMINATED"
	default:
		return "UNKNOWN"
	}
}

// Startable defines the interface for components with lifecycle management.
type Startable interface {
	Start() error
	Stop() error
	Name() string
	State() State
}

// AbstractStartable provides base lifecycle management functionality.
type AbstractStartable struct {
	name   string
	state  State
	mu     sync.RWMutex
	logger *slog.Logger

	// SetUp is called when Start() is invoked.
	SetUp func() error

	// TearDown is called when Stop() is invoked.
	TearDown func() error
}

// NewAbstractStartable creates a new AbstractStartable with the given name.
func NewAbstractStartable(name string) *AbstractStartable {
	return &AbstractStartable{
		name:   name,
		state:  StateReady,
		logger: slog.Default().With("component", name),
	}
}

// Name returns the name of this startable.
func (s *AbstractStartable) Name() string {
	return s.name
}

// State returns the current execution state.
func (s *AbstractStartable) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Logger returns the logger for this startable.
func (s *AbstractStartable) Logger() *slog.Logger {
	return s.logger
}

// Start starts the execution.
func (s *AbstractStartable) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateReady {
		panic("invalid state: " + s.state.String())
	}

	s.logger.Info("Starting", "name", s.name)
	s.state = StateRunning

	if s.SetUp != nil {
		if err := s.SetUp(); err != nil {
			s.state = StateTerminated
			return err
		}
	}

	return nil
}

// Stop stops the execution.
func (s *AbstractStartable) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateRunning {
		return nil
	}

	s.logger.Info("Stopping", "name", s.name)
	s.state = StateTerminated

	if s.TearDown != nil {
		return s.TearDown()
	}

	return nil
}
