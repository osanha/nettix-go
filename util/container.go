package util

import "sync/atomic"

// ObjectContainer wraps and holds an object.
type ObjectContainer[T any] struct {
	value *T
}

// NewObjectContainer creates an empty ObjectContainer.
func NewObjectContainer[T any]() *ObjectContainer[T] {
	return &ObjectContainer[T]{}
}

// NewObjectContainerWithValue creates an ObjectContainer with an initial value.
func NewObjectContainerWithValue[T any](value T) *ObjectContainer[T] {
	return &ObjectContainer[T]{value: &value}
}

// Set sets the contained object.
func (c *ObjectContainer[T]) Set(value T) {
	c.value = &value
}

// Get returns the contained object.
func (c *ObjectContainer[T]) Get() *T {
	return c.value
}

// Contains checks if an object is currently held.
func (c *ObjectContainer[T]) Contains() bool {
	return c.value != nil
}

// Remove removes and returns the held object.
func (c *ObjectContainer[T]) Remove() *T {
	val := c.value
	c.value = nil
	return val
}

// RoundRobinInteger is a thread-safe round-robin sequencer.
type RoundRobinInteger struct {
	min   int32
	max   int32
	value atomic.Int32
}

// NewRoundRobinInteger creates a new RoundRobinInteger with the given max (min defaults to 0).
func NewRoundRobinInteger(max int32) *RoundRobinInteger {
	return NewRoundRobinIntegerRange(0, max)
}

// NewRoundRobinIntegerRange creates a new RoundRobinInteger with min and max values.
func NewRoundRobinIntegerRange(min, max int32) *RoundRobinInteger {
	r := &RoundRobinInteger{
		min: min,
		max: max,
	}
	r.value.Store(min)
	return r
}

// Next returns the current value and increments it.
// Rolls over to minimum value if maximum is exceeded.
func (r *RoundRobinInteger) Next() int32 {
	for {
		current := r.value.Load()
		next := current + 1
		if next > r.max {
			next = r.min
		}
		if r.value.CompareAndSwap(current, next) {
			return current
		}
	}
}
