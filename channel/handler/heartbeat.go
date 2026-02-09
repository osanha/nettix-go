package handler

import (
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/osanha/nettix-go/util"
)

// HeartbeatFactory generates heartbeat messages.
type HeartbeatFactory[T any] interface {
	CreateHeartbeat() T
}

// HeartbeatHandler sends periodic heartbeat messages to maintain connection.
type HeartbeatHandler[T any] struct {
	name     string
	interval time.Duration
	timeout  time.Duration
	factory  HeartbeatFactory[T]
	logger   *slog.Logger
	tasks    sync.Map // map[net.Conn]*util.Timeout
	writeFn  func(conn net.Conn, msg T) error
}

// NewHeartbeatHandler creates a new HeartbeatHandler.
func NewHeartbeatHandler[T any](name string, interval, timeout time.Duration, factory HeartbeatFactory[T]) *HeartbeatHandler[T] {
	return &HeartbeatHandler[T]{
		name:     name,
		interval: interval,
		timeout:  timeout,
		factory:  factory,
		logger:   slog.Default().With("component", "HeartbeatHandler."+name),
	}
}

// SetWriteFunc sets the function used to write messages.
func (h *HeartbeatHandler[T]) SetWriteFunc(fn func(conn net.Conn, msg T) error) {
	h.writeFn = fn
}

// OnConnected starts sending heartbeat messages.
func (h *HeartbeatHandler[T]) OnConnected(conn net.Conn) error {
	h.start(conn)
	return nil
}

// OnDisconnected stops sending heartbeat messages.
func (h *HeartbeatHandler[T]) OnDisconnected(conn net.Conn) error {
	h.stop(conn)
	return nil
}

// OnMessage is a no-op for this handler.
func (h *HeartbeatHandler[T]) OnMessage(conn net.Conn, msg interface{}) error {
	return nil
}

// OnError is a no-op for this handler.
func (h *HeartbeatHandler[T]) OnError(conn net.Conn, err error) {}

func (h *HeartbeatHandler[T]) start(conn net.Conn) {
	var scheduleNext func()

	scheduleNext = func() {
		task := util.Singleton.Timer().NewTimeout(func() {
			heartbeat := h.factory.CreateHeartbeat()

			if h.writeFn != nil {
				if err := h.writeFn(conn, heartbeat); err != nil {
					h.logger.Error("Failed to send heartbeat", "conn", conn.RemoteAddr(), "error", err)
					conn.Close()
					return
				}
			}

			// Schedule next heartbeat
			scheduleNext()
		}, h.interval)

		h.tasks.Store(conn, task)
	}

	scheduleNext()
}

func (h *HeartbeatHandler[T]) stop(conn net.Conn) {
	if task, ok := h.tasks.LoadAndDelete(conn); ok {
		task.(*util.Timeout).Cancel()
	}
}

// ReadTimeoutHandler closes the channel if no data is received within timeout.
type ReadTimeoutHandler struct {
	timeout time.Duration
	logger  *slog.Logger
}

// NewReadTimeoutHandler creates a new ReadTimeoutHandler.
func NewReadTimeoutHandler(timeout time.Duration) *ReadTimeoutHandler {
	return &ReadTimeoutHandler{
		timeout: timeout,
		logger:  slog.Default().With("component", "ReadTimeoutHandler"),
	}
}

// SetReadDeadline sets the read deadline on the connection.
func (h *ReadTimeoutHandler) SetReadDeadline(conn net.Conn) error {
	return conn.SetReadDeadline(time.Now().Add(h.timeout))
}

// OnConnected sets the initial read deadline.
func (h *ReadTimeoutHandler) OnConnected(conn net.Conn) error {
	return h.SetReadDeadline(conn)
}

// OnDisconnected is a no-op.
func (h *ReadTimeoutHandler) OnDisconnected(conn net.Conn) error {
	return nil
}

// OnMessage resets the read deadline.
func (h *ReadTimeoutHandler) OnMessage(conn net.Conn, msg interface{}) error {
	return h.SetReadDeadline(conn)
}

// OnError logs and closes on timeout.
func (h *ReadTimeoutHandler) OnError(conn net.Conn, err error) {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		h.logger.Info("Read timeout, closing connection", "conn", conn.RemoteAddr())
		conn.Close()
	}
}

// EnquireLinkFactory creates enquire link (ping) messages.
type EnquireLinkFactory[T any] interface {
	// CreateEnquireLink creates an enquire link request message.
	CreateEnquireLink(sequence int32) T
}

// EnquireLinkHandler sends periodic enquire link messages and handles responses.
type EnquireLinkHandler[T any] struct {
	name      string
	interval  time.Duration
	timeout   time.Duration
	factory   EnquireLinkFactory[T]
	sequencer *util.RoundRobinInteger
	logger    *slog.Logger
	tasks     sync.Map // map[net.Conn]*util.Timeout
	writeFn   func(conn net.Conn, msg T) error
}

// NewEnquireLinkHandler creates a new EnquireLinkHandler.
func NewEnquireLinkHandler[T any](name string, interval, timeout time.Duration, factory EnquireLinkFactory[T], sequencer *util.RoundRobinInteger) *EnquireLinkHandler[T] {
	return &EnquireLinkHandler[T]{
		name:      name,
		interval:  interval,
		timeout:   timeout,
		factory:   factory,
		sequencer: sequencer,
		logger:    slog.Default().With("component", "EnquireLinkHandler."+name),
	}
}

// SetWriteFunc sets the function used to write messages.
func (h *EnquireLinkHandler[T]) SetWriteFunc(fn func(conn net.Conn, msg T) error) {
	h.writeFn = fn
}

// OnConnected starts sending enquire link messages.
func (h *EnquireLinkHandler[T]) OnConnected(conn net.Conn) error {
	h.start(conn)
	return nil
}

// OnDisconnected stops sending enquire link messages.
func (h *EnquireLinkHandler[T]) OnDisconnected(conn net.Conn) error {
	h.stop(conn)
	return nil
}

// OnMessage is a no-op for this handler.
func (h *EnquireLinkHandler[T]) OnMessage(conn net.Conn, msg interface{}) error {
	return nil
}

// OnError is a no-op for this handler.
func (h *EnquireLinkHandler[T]) OnError(conn net.Conn, err error) {}

func (h *EnquireLinkHandler[T]) start(conn net.Conn) {
	var scheduleNext func()

	scheduleNext = func() {
		task := util.Singleton.Timer().NewTimeout(func() {
			sequence := h.sequencer.Next()
			enquireLink := h.factory.CreateEnquireLink(sequence)

			if h.writeFn != nil {
				if err := h.writeFn(conn, enquireLink); err != nil {
					h.logger.Error("Failed to send enquire link", "conn", conn.RemoteAddr(), "error", err)
					conn.Close()
					return
				}
			}

			// Schedule next enquire link
			scheduleNext()
		}, h.interval)

		h.tasks.Store(conn, task)
	}

	scheduleNext()
}

func (h *EnquireLinkHandler[T]) stop(conn net.Conn) {
	if task, ok := h.tasks.LoadAndDelete(conn); ok {
		task.(*util.Timeout).Cancel()
	}
}
