package util

import (
	"log/slog"
	"sync"
	"time"
)

// TimeoutHandler handles timeout events.
type TimeoutHandler[K comparable, V any] func(key K, value V)

// TimeoutableMap is a map that handles object timeouts conveniently.
type TimeoutableMap[K comparable, V any] struct {
	name           string
	defaultTimeout time.Duration
	timeoutIsError bool
	handler        TimeoutHandler[K, V]
	items          sync.Map // map[K]*timeoutableEntry[V]
	logger         *slog.Logger
}

type timeoutableEntry[V any] struct {
	value   V
	timeout time.Duration
	task    *Timeout
}

// NewTimeoutableMap creates a new TimeoutableMap.
func NewTimeoutableMap[K comparable, V any](name string) *TimeoutableMap[K, V] {
	return &TimeoutableMap[K, V]{
		name:           name,
		timeoutIsError: true,
		logger:         slog.Default().With("component", "TimeoutableMap."+name),
	}
}

// NewTimeoutableMapWithTimeout creates a new TimeoutableMap with default timeout.
func NewTimeoutableMapWithTimeout[K comparable, V any](name string, timeout time.Duration) *TimeoutableMap[K, V] {
	m := NewTimeoutableMap[K, V](name)
	m.defaultTimeout = timeout
	return m
}

// SetTimeoutIsError sets whether timeouts should be logged at ERROR level.
func (m *TimeoutableMap[K, V]) SetTimeoutIsError(isError bool) {
	m.timeoutIsError = isError
}

// SetTimeoutHandler registers a timeout handler.
func (m *TimeoutableMap[K, V]) SetTimeoutHandler(handler TimeoutHandler[K, V]) {
	m.handler = handler
}

// Put adds an object to the map with the default timeout.
func (m *TimeoutableMap[K, V]) Put(key K, value V) *V {
	return m.PutWithTimeout(key, value, m.defaultTimeout)
}

// PutWithTimeout adds an object to the map with a specific timeout.
func (m *TimeoutableMap[K, V]) PutWithTimeout(key K, value V, timeout time.Duration) *V {
	entry := &timeoutableEntry[V]{
		value:   value,
		timeout: timeout,
	}

	if timeout > 0 {
		entry.task = m.newTimeout(key, entry)
	}

	if prev, loaded := m.items.Swap(key, entry); loaded {
		prevEntry := prev.(*timeoutableEntry[V])
		if prevEntry.task != nil {
			prevEntry.task.Cancel()
		}
		return &prevEntry.value
	}

	return nil
}

// PutIfAbsent adds an object if the key is absent.
func (m *TimeoutableMap[K, V]) PutIfAbsent(key K, value V) *V {
	return m.PutIfAbsentWithTimeout(key, value, m.defaultTimeout)
}

// PutIfAbsentWithTimeout adds an object if the key is absent with specific timeout.
func (m *TimeoutableMap[K, V]) PutIfAbsentWithTimeout(key K, value V, timeout time.Duration) *V {
	entry := &timeoutableEntry[V]{
		value:   value,
		timeout: timeout,
	}

	if actual, loaded := m.items.LoadOrStore(key, entry); loaded {
		return &actual.(*timeoutableEntry[V]).value
	}

	if timeout > 0 {
		entry.task = m.newTimeout(key, entry)
	}

	return nil
}

func (m *TimeoutableMap[K, V]) newTimeout(key K, entry *timeoutableEntry[V]) *Timeout {
	return Singleton.Timer().NewTimeout(func() {
		m.items.Delete(key)

		if m.timeoutIsError {
			m.logger.Error("Timed out", "key", key)
		} else {
			m.logger.Info("Timed out", "key", key)
		}

		if m.handler != nil {
			m.handler(key, entry.value)
		}
	}, entry.timeout)
}

// Get retrieves an object from the map.
func (m *TimeoutableMap[K, V]) Get(key K) *V {
	if entry, ok := m.items.Load(key); ok {
		return &entry.(*timeoutableEntry[V]).value
	}
	return nil
}

// Remove removes an object from the map.
func (m *TimeoutableMap[K, V]) Remove(key K) *V {
	if entry, loaded := m.items.LoadAndDelete(key); loaded {
		e := entry.(*timeoutableEntry[V])
		if e.task != nil {
			e.task.Cancel()
		}
		return &e.value
	}
	return nil
}

// ContainsKey checks whether a key exists in the map.
func (m *TimeoutableMap[K, V]) ContainsKey(key K) bool {
	_, ok := m.items.Load(key)
	return ok
}

// Cancel cancels an ongoing timeout for a specific key.
func (m *TimeoutableMap[K, V]) Cancel(key K) bool {
	if entry, ok := m.items.Load(key); ok {
		e := entry.(*timeoutableEntry[V])
		if e.task != nil {
			e.task.Cancel()
			e.task = nil
			return true
		}
	}
	return false
}

// Clear clears all objects from the map.
func (m *TimeoutableMap[K, V]) Clear() {
	m.items.Range(func(key, value any) bool {
		e := value.(*timeoutableEntry[V])
		if e.task != nil {
			e.task.Cancel()
		}
		m.items.Delete(key)
		return true
	})
}

// Size returns the number of stored objects.
func (m *TimeoutableMap[K, V]) Size() int {
	count := 0
	m.items.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
