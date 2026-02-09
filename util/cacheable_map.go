package util

import (
	"sync"
	"time"
)

// CacheableMap is a map for handling caching with LRU-style timeout refresh.
type CacheableMap[K comparable, V any] struct {
	*TimeoutableMap[K, V]
	lastAccessTimes sync.Map // map[K]time.Time
}

// NewCacheableMap creates a new CacheableMap.
func NewCacheableMap[K comparable, V any](name string) *CacheableMap[K, V] {
	m := &CacheableMap[K, V]{
		TimeoutableMap: NewTimeoutableMap[K, V](name),
	}
	m.SetTimeoutIsError(false)
	return m
}

// NewCacheableMapWithTimeout creates a new CacheableMap with default timeout.
func NewCacheableMapWithTimeout[K comparable, V any](name string, timeout time.Duration) *CacheableMap[K, V] {
	m := &CacheableMap[K, V]{
		TimeoutableMap: NewTimeoutableMapWithTimeout[K, V](name, timeout),
	}
	m.SetTimeoutIsError(false)
	return m
}

// Get retrieves an object from the cache, refreshing its last access time.
func (m *CacheableMap[K, V]) Get(key K) *V {
	val := m.TimeoutableMap.Get(key)
	if val != nil {
		m.lastAccessTimes.Store(key, time.Now())
	}
	return val
}

// Put adds an object to the cache.
func (m *CacheableMap[K, V]) Put(key K, value V) *V {
	m.lastAccessTimes.Store(key, time.Now())
	return m.TimeoutableMap.Put(key, value)
}

// PutWithTimeout adds an object to the cache with a specific timeout.
func (m *CacheableMap[K, V]) PutWithTimeout(key K, value V, timeout time.Duration) *V {
	m.lastAccessTimes.Store(key, time.Now())
	return m.TimeoutableMap.PutWithTimeout(key, value, timeout)
}

// Remove removes an object from the cache.
func (m *CacheableMap[K, V]) Remove(key K) *V {
	m.lastAccessTimes.Delete(key)
	return m.TimeoutableMap.Remove(key)
}

// Clear clears all objects from the cache.
func (m *CacheableMap[K, V]) Clear() {
	m.lastAccessTimes.Range(func(key, _ any) bool {
		m.lastAccessTimes.Delete(key)
		return true
	})
	m.TimeoutableMap.Clear()
}
