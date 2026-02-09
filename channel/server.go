package channel

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// ServerChannelManager manages server-side connections.
type ServerChannelManager struct {
	*AbstractChannelManager

	port     int
	options  ServerOptions
	listener net.Listener
	wg       sync.WaitGroup
	stopCh   chan struct{}

	// ConnHandler is called for each new connection.
	// It should handle the connection lifecycle (read/write loop).
	// If not set, uses the built-in pipeline-based handler.
	ConnHandler func(conn net.Conn)

	// ContextHandler is called for each new connection with a ChannelContext.
	// This is the preferred way to handle connections with pipeline support.
	// If set, takes precedence over ConnHandler.
	ContextHandler func(ctx *ChannelContext)

	// UsePipeline enables the built-in read loop with decoder/encoder support.
	// When true and no ContextHandler is set, connections are handled
	// using the pipeline's decoders, handlers, and encoders.
	usePipeline bool
}

// NewServerChannelManager creates a new server channel manager.
func NewServerChannelManager(name string, port int) *ServerChannelManager {
	m := &ServerChannelManager{
		AbstractChannelManager: NewAbstractChannelManager("Server." + name),
		port:                   port,
		options:                DefaultServerOptions(),
		stopCh:                 make(chan struct{}),
	}

	m.SetUp = m.setUp
	m.TearDown = m.tearDown

	return m
}

// Port returns the port this server listens on.
func (m *ServerChannelManager) Port() int {
	return m.port
}

// SetBacklog sets the connection backlog size.
func (m *ServerChannelManager) SetBacklog(backlog int) {
	m.options.Backlog = backlog
}

// SetOptions sets the server socket options.
func (m *ServerChannelManager) SetOptions(options ServerOptions) {
	m.options = options
}

// Listener returns the underlying listener.
func (m *ServerChannelManager) Listener() net.Listener {
	return m.listener
}

// UsePipeline enables the built-in pipeline-based read loop.
func (m *ServerChannelManager) UsePipeline(enabled bool) {
	m.usePipeline = enabled
}

func (m *ServerChannelManager) setUp() error {
	addr := fmt.Sprintf(":%d", m.port)

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return err
	}

	m.listener = listener
	m.Logger().Info("Binding to port", "port", m.port)

	// Start accept loop
	m.wg.Add(1)
	go m.acceptLoop()

	return nil
}

func (m *ServerChannelManager) tearDown() error {
	close(m.stopCh)

	if m.listener != nil {
		m.listener.Close()
	}

	// Close all managed connections
	if m.Connections() != nil {
		m.Connections().Close()
	}

	m.wg.Wait()
	return nil
}

func (m *ServerChannelManager) acceptLoop() {
	defer m.wg.Done()

	for {
		conn, err := m.listener.Accept()
		if err != nil {
			select {
			case <-m.stopCh:
				return
			default:
				m.Logger().Error("Accept error", "error", err)
				continue
			}
		}

		m.wg.Add(1)
		go m.handleConnection(conn)
	}
}

func (m *ServerChannelManager) handleConnection(conn net.Conn) {
	defer m.wg.Done()

	// Apply socket options
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := m.options.SocketOptions.Apply(tcpConn); err != nil {
			m.Logger().Error("Failed to apply socket options", "error", err)
		}
	}

	// Wrap with TLS if configured
	wrappedConn, err := m.WrapWithTLS(conn, true)
	if err != nil {
		m.Logger().Error("TLS handshake failed", "error", err)
		conn.Close()
		return
	}

	// Use ContextHandler if set (new pipeline-based approach)
	if m.ContextHandler != nil {
		ctx := m.NewChannelContext(wrappedConn)
		m.ContextHandler(ctx)
		return
	}

	// Use built-in pipeline read loop if enabled
	if m.usePipeline {
		ctx := m.NewChannelContext(wrappedConn)
		ctx.ReadLoop()
		return
	}

	// Legacy handler approach
	defer wrappedConn.Close()

	// Notify handlers of connection
	if err := m.NotifyConnected(wrappedConn); err != nil {
		m.Logger().Error("Connection handler error", "error", err)
		return
	}

	// Call user-defined connection handler
	if m.ConnHandler != nil {
		m.ConnHandler(wrappedConn)
	}

	// Notify handlers of disconnection
	m.NotifyDisconnected(wrappedConn)
}

// Broadcast sends a message to all connected clients.
// Requires UseChannelGroup(true) to be set.
func (m *ServerChannelManager) Broadcast(data []byte) error {
	if m.Connections() == nil {
		return fmt.Errorf("channel group not enabled")
	}
	return m.Connections().Broadcast(data)
}

// ServerChannel returns the underlying listener (for compatibility).
func (m *ServerChannelManager) ServerChannel() net.Listener {
	return m.listener
}
