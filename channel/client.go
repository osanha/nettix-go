package channel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"nettix-go/util"
)

var (
	// ErrConnectTimeout is returned when connection times out.
	ErrConnectTimeout = errors.New("connect timeout")
)

// ClientChannelManager manages client-side connections.
type ClientChannelManager struct {
	*AbstractChannelManager

	options        ClientOptions
	reconnCount    int
	reconnInterval time.Duration
	connTimeout    time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup

	// ConnHandler is called for each established connection.
	// It should handle the connection lifecycle (read/write loop).
	ConnHandler func(conn net.Conn)

	// ContextHandler is called for each established connection with a ChannelContext.
	// This is the preferred way to handle connections with pipeline support.
	// If set, takes precedence over ConnHandler.
	ContextHandler func(ctx *ChannelContext)

	// UsePipeline enables the built-in read loop with decoder/encoder support.
	usePipeline bool
}

// NewClientChannelManager creates a new client channel manager.
func NewClientChannelManager(name string) *ClientChannelManager {
	m := &ClientChannelManager{
		AbstractChannelManager: NewAbstractChannelManager("Client." + name),
		options:                DefaultClientOptions(),
		reconnCount:            2,
		reconnInterval:         1 * time.Second,
		connTimeout:            30 * time.Second,
		stopCh:                 make(chan struct{}),
	}

	m.SetUp = m.setUp
	m.TearDown = m.tearDown

	return m
}

// SetReconnCount sets the number of reconnection attempts.
// Use -1 for infinite retries.
func (m *ClientChannelManager) SetReconnCount(count int) {
	m.reconnCount = count
}

// SetConnInterval sets the interval between reconnection attempts.
func (m *ClientChannelManager) SetConnInterval(interval time.Duration) {
	if interval <= 0 {
		panic("interval must be positive")
	}
	m.reconnInterval = interval
}

// SetConnTimeout sets the connection timeout.
func (m *ClientChannelManager) SetConnTimeout(timeout time.Duration) {
	if timeout <= 0 {
		panic("timeout must be positive")
	}
	m.connTimeout = timeout
}

// SetOptions sets the client socket options.
func (m *ClientChannelManager) SetOptions(options ClientOptions) {
	m.options = options
}

// UsePipeline enables the built-in pipeline-based read loop.
func (m *ClientChannelManager) UsePipeline(enabled bool) {
	m.usePipeline = enabled
}

func (m *ClientChannelManager) setUp() error {
	return nil
}

func (m *ClientChannelManager) tearDown() error {
	close(m.stopCh)

	// Close all managed connections
	if m.Connections() != nil {
		m.Connections().Close()
	}

	m.wg.Wait()
	return nil
}

// Connect asynchronously connects to the specified host and port.
func (m *ClientChannelManager) Connect(host string, port int) *Future {
	return m.ConnectAddr(fmt.Sprintf("%s:%d", host, port))
}

// ConnectAddr asynchronously connects to the specified address.
func (m *ClientChannelManager) ConnectAddr(addr string) *Future {
	if m.State() != util.StateRunning {
		future := NewFuture()
		future.SetFailure(fmt.Errorf("invalid state: %s", m.State()))
		return future
	}

	future := NewFuture()
	go m.connectWithRetry(addr, future)
	return future
}

// ConnectSync synchronously connects to the specified address.
func (m *ClientChannelManager) ConnectSync(host string, port int) (*ChannelContext, error) {
	future := m.Connect(host, port)
	future.Await()
	if future.IsSuccess() {
		return m.NewChannelContext(future.Conn()), nil
	}
	return nil, future.Cause()
}

func (m *ClientChannelManager) connectWithRetry(addr string, future *Future) {
	var lastErr error
	attemptsLeft := m.reconnCount

	for {
		select {
		case <-m.stopCh:
			future.SetFailure(errors.New("client stopped"))
			return
		default:
		}

		conn, err := m.dial(addr)
		if err == nil {
			future.SetConn(conn)
			future.SetSuccess()

			// Handle connection
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				m.handleConnection(conn)
			}()
			return
		}

		lastErr = err

		// Check if we should retry
		if errors.Is(err, ErrConnectTimeout) || attemptsLeft == 0 {
			future.SetFailure(lastErr)
			return
		}

		if attemptsLeft > 0 {
			attemptsLeft--
		}

		// Wait before retry
		select {
		case <-m.stopCh:
			future.SetFailure(errors.New("client stopped"))
			return
		case <-time.After(m.reconnInterval):
			// Continue retry loop
		}
	}
}

func (m *ClientChannelManager) dial(addr string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.connTimeout)
	defer cancel()

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrConnectTimeout
		}
		return nil, err
	}

	// Apply socket options
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := m.options.SocketOptions.Apply(tcpConn); err != nil {
			conn.Close()
			return nil, err
		}
	}

	// Wrap with TLS if configured
	wrappedConn, err := m.WrapWithTLS(conn, false)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Notify handlers (legacy mode)
	if !m.usePipeline && m.ContextHandler == nil {
		if err := m.NotifyConnected(wrappedConn); err != nil {
			wrappedConn.Close()
			return nil, err
		}
	}

	return wrappedConn, nil
}

func (m *ClientChannelManager) handleConnection(conn net.Conn) {
	// Use ContextHandler if set (new pipeline-based approach)
	if m.ContextHandler != nil {
		ctx := m.NewChannelContext(conn)
		m.ContextHandler(ctx)
		return
	}

	// Use built-in pipeline read loop if enabled
	if m.usePipeline {
		ctx := m.NewChannelContext(conn)
		ctx.ReadLoop()
		return
	}

	// Legacy handler approach
	defer conn.Close()

	if m.ConnHandler != nil {
		m.ConnHandler(conn)
	}

	m.NotifyDisconnected(conn)
}
