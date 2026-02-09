package channel

import (
	"io"
	"log/slog"
	"net"
	"sync"
)

// LifecycleHandler handles connection lifecycle events.
type LifecycleHandler interface {
	// OnConnected is called when a connection is established.
	OnConnected(ctx *ChannelContext) error
	// OnDisconnected is called when a connection is closed.
	OnDisconnected(ctx *ChannelContext) error
}

// InboundHandler handles inbound messages.
type InboundHandler interface {
	// OnMessage is called when a message is received.
	// The handler can return a transformed message to pass to the next handler,
	// or nil to stop propagation.
	OnMessage(ctx *ChannelContext, msg interface{}) (interface{}, error)
}

// OutboundHandler handles outbound messages.
type OutboundHandler interface {
	// OnWrite is called when a message is about to be sent.
	// The handler can return a transformed message to pass to the next handler,
	// or nil to stop propagation.
	OnWrite(ctx *ChannelContext, msg interface{}) (interface{}, error)
}

// ErrorHandler handles errors.
type ErrorHandler interface {
	// OnError is called when an error occurs.
	OnError(ctx *ChannelContext, err error)
}

// ChannelHandler combines all handler interfaces.
type ChannelHandler interface {
	LifecycleHandler
	InboundHandler
	OutboundHandler
	ErrorHandler
}

// BaseChannelHandler provides default implementations for ChannelHandler.
type BaseChannelHandler struct{}

func (h *BaseChannelHandler) OnConnected(ctx *ChannelContext) error               { return nil }
func (h *BaseChannelHandler) OnDisconnected(ctx *ChannelContext) error            { return nil }
func (h *BaseChannelHandler) OnMessage(ctx *ChannelContext, msg interface{}) (interface{}, error) { return msg, nil }
func (h *BaseChannelHandler) OnWrite(ctx *ChannelContext, msg interface{}) (interface{}, error)   { return msg, nil }
func (h *BaseChannelHandler) OnError(ctx *ChannelContext, err error)              {}

// ChannelContext represents the context of a single connection.
// It provides access to the connection, pipeline, and methods for reading/writing.
type ChannelContext struct {
	conn       net.Conn
	pipeline   *Pipeline
	readBuffer *Buffer
	attrs      sync.Map
	logger     *slog.Logger
	manager    ChannelManager
	closed     bool
	closeMu    sync.Mutex
}

// ChannelManager is the interface for channel managers.
type ChannelManager interface {
	Logger() *slog.Logger
	GetPipeline() *Pipeline
}

// NewChannelContext creates a new channel context.
func NewChannelContext(conn net.Conn, manager ChannelManager) *ChannelContext {
	return &ChannelContext{
		conn:       conn,
		pipeline:   manager.GetPipeline(),
		readBuffer: NewEmptyBuffer(),
		logger:     manager.Logger(),
		manager:    manager,
	}
}

// Conn returns the underlying connection.
func (ctx *ChannelContext) Conn() net.Conn {
	return ctx.conn
}

// Pipeline returns the pipeline for this context.
func (ctx *ChannelContext) Pipeline() *Pipeline {
	return ctx.pipeline
}

// Logger returns the logger.
func (ctx *ChannelContext) Logger() *slog.Logger {
	return ctx.logger
}

// RemoteAddr returns the remote address.
func (ctx *ChannelContext) RemoteAddr() net.Addr {
	return ctx.conn.RemoteAddr()
}

// LocalAddr returns the local address.
func (ctx *ChannelContext) LocalAddr() net.Addr {
	return ctx.conn.LocalAddr()
}

// SetAttr sets an attribute on this context.
func (ctx *ChannelContext) SetAttr(key string, value interface{}) {
	ctx.attrs.Store(key, value)
}

// GetAttr gets an attribute from this context.
func (ctx *ChannelContext) GetAttr(key string) (interface{}, bool) {
	return ctx.attrs.Load(key)
}

// RemoveAttr removes an attribute from this context.
func (ctx *ChannelContext) RemoveAttr(key string) {
	ctx.attrs.Delete(key)
}

// Close closes the connection.
func (ctx *ChannelContext) Close() error {
	ctx.closeMu.Lock()
	if ctx.closed {
		ctx.closeMu.Unlock()
		return nil
	}
	ctx.closed = true
	ctx.closeMu.Unlock()
	return ctx.conn.Close()
}

// IsClosed returns whether the connection is closed.
func (ctx *ChannelContext) IsClosed() bool {
	ctx.closeMu.Lock()
	defer ctx.closeMu.Unlock()
	return ctx.closed
}

// Write writes a message through the outbound pipeline and sends it.
func (ctx *ChannelContext) Write(msg interface{}) *Future {
	future := NewFuture()

	go func() {
		// Process through outbound handlers (in reverse order)
		outboundHandlers := ctx.pipeline.OutboundHandlers()
		currentMsg := msg
		var err error

		for i := len(outboundHandlers) - 1; i >= 0; i-- {
			currentMsg, err = outboundHandlers[i].OnWrite(ctx, currentMsg)
			if err != nil {
				future.SetFailure(err)
				return
			}
			if currentMsg == nil {
				future.SetSuccess() // Handler consumed the message
				return
			}
		}

		// Encode through encoders (in reverse order)
		encoders := ctx.pipeline.Encoders()
		for i := len(encoders) - 1; i >= 0; i-- {
			encoded, err := encoders[i].Encode(ctx, currentMsg)
			if err != nil {
				future.SetFailure(err)
				return
			}
			currentMsg = encoded
		}

		// Write to connection
		data, ok := currentMsg.([]byte)
		if !ok {
			future.SetFailure(ErrNotEnoughData)
			return
		}

		_, err = ctx.conn.Write(data)
		if err != nil {
			future.SetFailure(err)
			return
		}

		future.SetSuccess()
	}()

	return future
}

// WriteSync writes a message synchronously.
func (ctx *ChannelContext) WriteSync(msg interface{}) error {
	future := ctx.Write(msg)
	future.Await()
	return future.Cause()
}

// FireConnected fires the connected event through the pipeline.
func (ctx *ChannelContext) FireConnected() error {
	for _, h := range ctx.pipeline.LifecycleHandlers() {
		if err := h.OnConnected(ctx); err != nil {
			return err
		}
	}
	return nil
}

// FireDisconnected fires the disconnected event through the pipeline.
func (ctx *ChannelContext) FireDisconnected() error {
	for _, h := range ctx.pipeline.LifecycleHandlers() {
		if err := h.OnDisconnected(ctx); err != nil {
			return err
		}
	}
	return nil
}

// FireMessage fires a message through the inbound pipeline.
func (ctx *ChannelContext) FireMessage(msg interface{}) error {
	currentMsg := msg

	for _, h := range ctx.pipeline.InboundHandlers() {
		var err error
		currentMsg, err = h.OnMessage(ctx, currentMsg)
		if err != nil {
			ctx.FireError(err)
			return err
		}
		if currentMsg == nil {
			return nil // Handler consumed the message
		}
	}
	return nil
}

// FireError fires an error through the pipeline.
func (ctx *ChannelContext) FireError(err error) {
	for _, h := range ctx.pipeline.Handlers() {
		if eh, ok := h.(ErrorHandler); ok {
			eh.OnError(ctx, err)
		}
	}
}

// ReadLoop runs the read loop, decoding messages and firing them through the pipeline.
func (ctx *ChannelContext) ReadLoop() {
	defer func() {
		ctx.FireDisconnected()
		ctx.Close()
	}()

	if err := ctx.FireConnected(); err != nil {
		ctx.logger.Error("Error in connected handler", "error", err)
		return
	}

	readBuf := make([]byte, 4096)

	for {
		n, err := ctx.conn.Read(readBuf)
		if err != nil {
			if err != io.EOF {
				ctx.FireError(err)
			}
			return
		}

		if n > 0 {
			ctx.readBuffer.Write(readBuf[:n])

			// Decode messages
			if err := ctx.decodeAndFire(); err != nil {
				ctx.FireError(err)
				return
			}
		}
	}
}

// decodeAndFire decodes messages from the read buffer and fires them through the pipeline.
func (ctx *ChannelContext) decodeAndFire() error {
	decoders := ctx.pipeline.Decoders()
	if len(decoders) == 0 {
		// No decoders - pass raw bytes
		data := ctx.readBuffer.Bytes()
		if len(data) > 0 {
			msg := make([]byte, len(data))
			copy(msg, data)
			ctx.readBuffer.Compact()
			return ctx.FireMessage(msg)
		}
		return nil
	}

	for ctx.readBuffer.ReadableBytes() > 0 {
		mark := ctx.readBuffer.MarkReaderIndex()
		var msg interface{} = nil
		var err error

		// Chain through decoders
		for _, decoder := range decoders {
			if msg == nil {
				msg, err = decoder.Decode(ctx, ctx.readBuffer)
			} else {
				// For chained decoders, create a buffer from the previous result
				if data, ok := msg.([]byte); ok {
					tempBuf := NewBuffer(data)
					msg, err = decoder.Decode(ctx, tempBuf)
				}
			}

			if err != nil {
				return err
			}
			if msg == nil {
				// Need more data, reset buffer position
				ctx.readBuffer.ResetReaderIndex(mark)
				ctx.readBuffer.Compact()
				return nil
			}
		}

		if msg != nil {
			if err := ctx.FireMessage(msg); err != nil {
				return err
			}
		}
	}

	ctx.readBuffer.Compact()
	return nil
}

// ConnectStateHandler handles only connection lifecycle events (adapter).
type ConnectStateHandler struct {
	OnConnectedFn    func(ctx *ChannelContext) error
	OnDisconnectedFn func(ctx *ChannelContext) error
}

func (h *ConnectStateHandler) OnConnected(ctx *ChannelContext) error {
	if h.OnConnectedFn != nil {
		return h.OnConnectedFn(ctx)
	}
	return nil
}

func (h *ConnectStateHandler) OnDisconnected(ctx *ChannelContext) error {
	if h.OnDisconnectedFn != nil {
		return h.OnDisconnectedFn(ctx)
	}
	return nil
}

// MessageHandlerFunc is an adapter for simple message handlers.
type MessageHandlerFunc func(ctx *ChannelContext, msg interface{}) error

// OnMessage implements InboundHandler.
func (f MessageHandlerFunc) OnMessage(ctx *ChannelContext, msg interface{}) (interface{}, error) {
	err := f(ctx, msg)
	return nil, err // Consume the message
}

// PassthroughHandler passes messages through without modification.
type PassthroughHandler struct {
	BaseChannelHandler
}

// LoggingInboundHandler logs all inbound messages.
type LoggingInboundHandler struct {
	logger *slog.Logger
}

// NewLoggingInboundHandler creates a new logging handler.
func NewLoggingInboundHandler(name string) *LoggingInboundHandler {
	return &LoggingInboundHandler{
		logger: slog.Default().With("component", "InboundLogger."+name),
	}
}

func (h *LoggingInboundHandler) OnMessage(ctx *ChannelContext, msg interface{}) (interface{}, error) {
	h.logger.Debug("Received message", "remote", ctx.RemoteAddr(), "msg", msg)
	return msg, nil
}

// LoggingOutboundHandler logs all outbound messages.
type LoggingOutboundHandler struct {
	logger *slog.Logger
}

// NewLoggingOutboundHandler creates a new logging handler.
func NewLoggingOutboundHandler(name string) *LoggingOutboundHandler {
	return &LoggingOutboundHandler{
		logger: slog.Default().With("component", "OutboundLogger."+name),
	}
}

func (h *LoggingOutboundHandler) OnWrite(ctx *ChannelContext, msg interface{}) (interface{}, error) {
	h.logger.Debug("Sending message", "remote", ctx.RemoteAddr(), "msg", msg)
	return msg, nil
}
