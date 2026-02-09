package channel

import (
	"crypto/tls"
	"log/slog"
	"net"
	"time"

	"github.com/osanha/nettix-go/channel/handler"
	"github.com/osanha/nettix-go/log"
	"github.com/osanha/nettix-go/ssl"
	"github.com/osanha/nettix-go/util"
)

// PipelineFactory creates a new pipeline for each connection.
// This allows per-connection handlers while sharing stateless handlers.
type PipelineFactory func() *Pipeline

// AbstractChannelManager provides common functionality for channel managers.
type AbstractChannelManager struct {
	*util.AbstractStartable

	tlsFactory      ssl.TLSConfigFactory
	sslTimeout      time.Duration
	channelGroup    *handler.ChannelGroupHandler
	useChannelGroup bool
	ioLogger        *log.LoggingHandler
	pipelineFactory PipelineFactory
	basePipeline    *Pipeline
	handshaker      *ssl.Handshaker

	// Legacy handler support
	legacyHandlers []handler.Handler
}

// NewAbstractChannelManager creates a new AbstractChannelManager.
func NewAbstractChannelManager(name string) *AbstractChannelManager {
	m := &AbstractChannelManager{
		AbstractStartable: util.NewAbstractStartable(name),
		sslTimeout:        30 * time.Second,
		ioLogger:          log.NewLoggingHandlerWithSuffix(name),
		handshaker:        ssl.NewHandshakerWithName(name),
		basePipeline:      NewPipeline(),
	}
	return m
}

// SetTLSFactory sets the TLS configuration factory.
func (m *AbstractChannelManager) SetTLSFactory(factory ssl.TLSConfigFactory) {
	m.tlsFactory = factory
}

// SetSSLHandshakeTimeout sets the SSL handshake timeout.
func (m *AbstractChannelManager) SetSSLHandshakeTimeout(timeout time.Duration) {
	if timeout <= 0 {
		panic("timeout must be positive")
	}
	m.sslTimeout = timeout
	m.handshaker.SetTimeout(timeout)
}

// UseChannelGroup enables or disables channel group management.
func (m *AbstractChannelManager) UseChannelGroup(enabled bool) {
	m.useChannelGroup = enabled
	if enabled {
		m.channelGroup = handler.NewChannelGroupHandler()
	} else {
		m.channelGroup = nil
	}
}

// Connections returns the channel group if enabled.
func (m *AbstractChannelManager) Connections() *handler.ChannelGroup {
	if m.channelGroup != nil {
		return m.channelGroup.Connections()
	}
	return nil
}

// IoLogger returns the IO logging handler.
func (m *AbstractChannelManager) IoLogger() *log.LoggingHandler {
	return m.ioLogger
}

// SetPipelineFactory sets the factory for creating handler pipelines.
// This is called for each new connection to create a per-connection pipeline.
func (m *AbstractChannelManager) SetPipelineFactory(factory PipelineFactory) {
	m.pipelineFactory = factory
}

// BasePipeline returns the base pipeline that is cloned for each connection.
// Add shared (stateless) handlers to this pipeline.
func (m *AbstractChannelManager) BasePipeline() *Pipeline {
	return m.basePipeline
}

// GetPipeline creates a new pipeline for a connection.
// It clones the base pipeline and adds framework handlers.
func (m *AbstractChannelManager) GetPipeline() *Pipeline {
	var pipeline *Pipeline

	if m.pipelineFactory != nil {
		pipeline = m.pipelineFactory()
	} else {
		pipeline = m.basePipeline.Clone()
	}

	// Add IO logger at the beginning
	pipeline.AddFirst("IO_LOGGER", &ioLoggerAdapter{m.ioLogger})

	// Add channel group handler at the end if enabled
	if m.channelGroup != nil {
		pipeline.AddLast("CHANNEL_GROUP", &channelGroupAdapter{m.channelGroup})
	}

	return pipeline
}

// AddHandler adds a legacy handler to the pipeline.
// Deprecated: Use BasePipeline().AddLast() instead.
func (m *AbstractChannelManager) AddHandler(h handler.Handler) {
	m.legacyHandlers = append(m.legacyHandlers, h)
}

// GetHandlers returns all legacy handlers.
// Deprecated: Use GetPipeline() instead.
func (m *AbstractChannelManager) GetHandlers() []handler.Handler {
	var handlers []handler.Handler
	handlers = append(handlers, m.legacyHandlers...)
	if m.channelGroup != nil {
		handlers = append(handlers, m.channelGroup)
	}
	return handlers
}

// WrapWithTLS wraps a connection with TLS if configured.
func (m *AbstractChannelManager) WrapWithTLS(conn net.Conn, isServer bool) (net.Conn, error) {
	if m.tlsFactory == nil {
		return conn, nil
	}

	config := m.tlsFactory.CreateTLSConfig()

	if isServer {
		return m.handshaker.HandshakeServer(conn, config)
	}
	return m.handshaker.HandshakeClient(conn, config)
}

// NotifyConnected notifies all legacy handlers of a new connection.
func (m *AbstractChannelManager) NotifyConnected(conn net.Conn) error {
	m.ioLogger.LogConnect(conn)
	for _, h := range m.GetHandlers() {
		if err := h.OnConnected(conn); err != nil {
			return err
		}
	}
	return nil
}

// NotifyDisconnected notifies all legacy handlers of a disconnection.
func (m *AbstractChannelManager) NotifyDisconnected(conn net.Conn) error {
	m.ioLogger.LogDisconnect(conn)
	for _, h := range m.GetHandlers() {
		if err := h.OnDisconnected(conn); err != nil {
			return err
		}
	}
	return nil
}

// NotifyMessage notifies all legacy handlers of a received message.
func (m *AbstractChannelManager) NotifyMessage(conn net.Conn, msg interface{}) error {
	for _, h := range m.GetHandlers() {
		if err := h.OnMessage(conn, msg); err != nil {
			return err
		}
	}
	return nil
}

// NotifyError notifies all legacy handlers of an error.
func (m *AbstractChannelManager) NotifyError(conn net.Conn, err error) {
	m.ioLogger.LogError(conn, err)
	for _, h := range m.GetHandlers() {
		h.OnError(conn, err)
	}
}

// TLSConfig returns the TLS configuration if available.
func (m *AbstractChannelManager) TLSConfig() *tls.Config {
	if m.tlsFactory != nil {
		return m.tlsFactory.CreateTLSConfig()
	}
	return nil
}

// ioLoggerAdapter adapts LoggingHandler to LifecycleHandler.
type ioLoggerAdapter struct {
	logger *log.LoggingHandler
}

func (a *ioLoggerAdapter) OnConnected(ctx *ChannelContext) error {
	a.logger.LogConnect(ctx.Conn())
	return nil
}

func (a *ioLoggerAdapter) OnDisconnected(ctx *ChannelContext) error {
	a.logger.LogDisconnect(ctx.Conn())
	return nil
}

// channelGroupAdapter adapts ChannelGroupHandler to the new interface.
type channelGroupAdapter struct {
	handler *handler.ChannelGroupHandler
}

func (a *channelGroupAdapter) OnConnected(ctx *ChannelContext) error {
	return a.handler.OnConnected(ctx.Conn())
}

func (a *channelGroupAdapter) OnDisconnected(ctx *ChannelContext) error {
	return a.handler.OnDisconnected(ctx.Conn())
}

// ChannelContextFactory creates a ChannelContext for a connection.
func (m *AbstractChannelManager) NewChannelContext(conn net.Conn) *ChannelContext {
	return NewChannelContext(conn, m)
}

// Logger returns the logger for this manager.
func (m *AbstractChannelManager) Logger() *slog.Logger {
	return m.AbstractStartable.Logger()
}
