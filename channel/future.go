package channel

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Future represents the result of an asynchronous operation.
type Future struct {
	conn      net.Conn
	done      chan struct{}
	err       error
	cancelled atomic.Bool
	mu        sync.Mutex
	listeners []func(*Future)
}

// NewFuture creates a new Future.
func NewFuture() *Future {
	return &Future{
		done: make(chan struct{}),
	}
}

// NewFutureWithConn creates a new Future with an associated connection.
func NewFutureWithConn(conn net.Conn) *Future {
	return &Future{
		conn: conn,
		done: make(chan struct{}),
	}
}

// NewFailedFuture creates a Future that has already failed.
func NewFailedFuture(conn net.Conn, err error) *Future {
	f := &Future{
		conn: conn,
		err:  err,
		done: make(chan struct{}),
	}
	close(f.done)
	return f
}

// Conn returns the connection associated with this future.
func (f *Future) Conn() net.Conn {
	return f.conn
}

// SetConn sets the connection for this future.
func (f *Future) SetConn(conn net.Conn) {
	f.conn = conn
}

// IsDone returns true if the operation has completed.
func (f *Future) IsDone() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// IsSuccess returns true if the operation completed successfully.
func (f *Future) IsSuccess() bool {
	return f.IsDone() && f.err == nil && !f.cancelled.Load()
}

// IsCancelled returns true if the operation was cancelled.
func (f *Future) IsCancelled() bool {
	return f.cancelled.Load()
}

// Cause returns the error that caused the failure, if any.
func (f *Future) Cause() error {
	return f.err
}

// SetSuccess marks the operation as successful.
func (f *Future) SetSuccess() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.done:
		return false
	default:
		close(f.done)
		f.notifyListeners()
		return true
	}
}

// SetFailure marks the operation as failed.
func (f *Future) SetFailure(err error) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.done:
		return false
	default:
		f.err = err
		close(f.done)
		f.notifyListeners()
		return true
	}
}

// Cancel cancels the operation.
func (f *Future) Cancel() bool {
	if f.cancelled.CompareAndSwap(false, true) {
		f.mu.Lock()
		defer f.mu.Unlock()

		select {
		case <-f.done:
			return false
		default:
			close(f.done)
			f.notifyListeners()
			return true
		}
	}
	return false
}

// AddListener adds a listener to be notified when the operation completes.
func (f *Future) AddListener(listener func(*Future)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.done:
		go listener(f)
	default:
		f.listeners = append(f.listeners, listener)
	}
}

// Await blocks until the operation completes.
func (f *Future) Await() {
	<-f.done
}

// AwaitTimeout blocks until the operation completes or timeout occurs.
func (f *Future) AwaitTimeout(timeout time.Duration) bool {
	select {
	case <-f.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (f *Future) notifyListeners() {
	for _, listener := range f.listeners {
		go listener(f)
	}
}

// CallableFuture is a Future that can carry a return value.
type CallableFuture[T any] struct {
	*Future
	value T
}

// NewCallableFuture creates a new CallableFuture.
func NewCallableFuture[T any]() *CallableFuture[T] {
	return &CallableFuture[T]{
		Future: NewFuture(),
	}
}

// NewCallableFutureWithConn creates a CallableFuture with a connection.
func NewCallableFutureWithConn[T any](conn net.Conn) *CallableFuture[T] {
	return &CallableFuture[T]{
		Future: NewFutureWithConn(conn),
	}
}

// SetSuccessWithValue marks the operation as successful with a value.
func (f *CallableFuture[T]) SetSuccessWithValue(value T) bool {
	f.value = value
	return f.Future.SetSuccess()
}

// Get returns the result value, blocking until complete.
func (f *CallableFuture[T]) Get() T {
	f.Await()
	return f.value
}

// GetTimeout returns the result value, blocking until complete or timeout.
func (f *CallableFuture[T]) GetTimeout(timeout time.Duration) (T, bool) {
	if f.AwaitTimeout(timeout) {
		return f.value, true
	}
	var zero T
	return zero, false
}

// CallableFutureListener is a listener for CallableFuture completion.
type CallableFutureListener[T any] func(*CallableFuture[T])

// AddCallableListener adds a typed listener to the future.
func (f *CallableFuture[T]) AddCallableListener(listener CallableFutureListener[T]) {
	f.Future.AddListener(func(future *Future) {
		listener(f)
	})
}

// CollectionFuture aggregates multiple futures.
type CollectionFuture struct {
	*Future
	counter atomic.Int32
}

// NewCollectionFuture creates a CollectionFuture from multiple futures.
func NewCollectionFuture(futures []*Future) *CollectionFuture {
	cf := &CollectionFuture{
		Future: NewFuture(),
	}

	if len(futures) == 0 {
		cf.SetSuccess()
		return cf
	}

	cf.counter.Store(int32(len(futures)))

	for _, f := range futures {
		f.AddListener(func(future *Future) {
			if future.IsSuccess() {
				if cf.counter.Add(-1) == 0 {
					cf.SetSuccess()
				}
			} else {
				cf.SetFailure(future.Cause())
			}
		})
	}

	return cf
}

// EmptyCollectionFuture returns an already completed empty collection future.
var EmptyCollectionFuture = func() *CollectionFuture {
	cf := &CollectionFuture{Future: NewFuture()}
	cf.SetSuccess()
	return cf
}()
