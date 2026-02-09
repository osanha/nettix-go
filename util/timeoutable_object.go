package util

import (
	"sync"
	"time"
)

// ObjectTimeoutHandler handles timeout events for a single object.
type ObjectTimeoutHandler[T any] func(value T)

// TimeoutableObject wraps and handles timeout for a single object.
type TimeoutableObject[T any] struct {
	defaultDelay time.Duration
	handler      ObjectTimeoutHandler[T]
	value        *T
	timeout      *Timeout
	mu           sync.Mutex
}

// NewTimeoutableObject creates a new TimeoutableObject.
func NewTimeoutableObject[T any]() *TimeoutableObject[T] {
	return &TimeoutableObject[T]{}
}

// NewTimeoutableObjectWithDelay creates a new TimeoutableObject with default delay.
func NewTimeoutableObjectWithDelay[T any](delay time.Duration) *TimeoutableObject[T] {
	if delay <= 0 {
		panic("delay must be positive")
	}
	return &TimeoutableObject[T]{
		defaultDelay: delay,
	}
}

// SetTimeoutHandler sets the timeout handler.
func (o *TimeoutableObject[T]) SetTimeoutHandler(handler ObjectTimeoutHandler[T]) {
	o.handler = handler
}

// Set sets the object with the default timeout.
func (o *TimeoutableObject[T]) Set(value T) {
	o.SetWithDelay(value, o.defaultDelay)
}

// SetWithDelay sets the object with a specific timeout duration.
func (o *TimeoutableObject[T]) SetWithDelay(value T, delay time.Duration) {
	if delay <= 0 {
		panic("delay must be positive")
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.cancel()
	o.value = &value

	o.timeout = Singleton.Timer().NewTimeout(func() {
		o.mu.Lock()
		val := o.value
		o.value = nil
		o.timeout = nil
		handler := o.handler
		o.mu.Unlock()

		if handler != nil && val != nil {
			handler(*val)
		}
	}, delay)
}

// SetWithHandler sets the object with a specific handler and timeout.
func (o *TimeoutableObject[T]) SetWithHandler(value T, handler ObjectTimeoutHandler[T], delay time.Duration) {
	o.handler = handler
	o.SetWithDelay(value, delay)
}

// Cancel cancels the active timeout.
func (o *TimeoutableObject[T]) Cancel() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.cancel()
}

func (o *TimeoutableObject[T]) cancel() bool {
	if o.timeout != nil {
		o.timeout.Cancel()
		o.timeout = nil
		return true
	}
	return false
}

// Remove removes and returns the managed object.
func (o *TimeoutableObject[T]) Remove() *T {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.cancel()
	val := o.value
	o.value = nil
	return val
}

// Contains checks if the managed object exists.
func (o *TimeoutableObject[T]) Contains() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.value != nil
}

// Get returns the managed object without removing it.
func (o *TimeoutableObject[T]) Get() *T {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.value
}
