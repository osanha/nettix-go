package channel

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	// ErrAlreadyConnecting is returned when already connecting to an address.
	ErrAlreadyConnecting = errors.New("already connecting to this address")
	// ErrClientStopped is returned when the client is stopped.
	ErrClientStopped = errors.New("client stopped")
)

// PersistentClientChannelManager manages client channels with automatic reconnection.
type PersistentClientChannelManager struct {
	*ClientChannelManager

	reconnDelay time.Duration
	activeConns sync.Map // map[string]bool
}

// NewPersistentClientChannelManager creates a new persistent client channel manager.
func NewPersistentClientChannelManager(name string) *PersistentClientChannelManager {
	cm := NewClientChannelManager(name)
	cm.SetReconnCount(-1) // Infinite retries

	return &PersistentClientChannelManager{
		ClientChannelManager: cm,
		reconnDelay:          1 * time.Second,
	}
}

// SetReconnDelay sets the delay before attempting reconnection.
func (m *PersistentClientChannelManager) SetReconnDelay(delay time.Duration) {
	if delay <= 0 {
		panic("delay must be positive")
	}
	m.reconnDelay = delay
}

// Connect connects to the specified host and port, maintaining the connection.
func (m *PersistentClientChannelManager) Connect(host string, port int) *Future {
	return m.ConnectAddr(fmt.Sprintf("%s:%d", host, port))
}

// ConnectAddr connects to the specified address, maintaining the connection.
func (m *PersistentClientChannelManager) ConnectAddr(addr string) *Future {
	future := NewFuture()
	go m.persistentConnect(addr, future, true)
	return future
}

func (m *PersistentClientChannelManager) persistentConnect(addr string, future *Future, isInitial bool) {
	// Check if already connecting
	if _, loaded := m.activeConns.LoadOrStore(addr, true); loaded && isInitial {
		future.SetFailure(ErrAlreadyConnecting)
		return
	}

	defer func() {
		if !isInitial {
			m.activeConns.Delete(addr)
		}
	}()

	for {
		select {
		case <-m.stopCh:
			if isInitial {
				future.SetFailure(ErrClientStopped)
			}
			m.activeConns.Delete(addr)
			return
		default:
		}

		conn, err := m.dial(addr)
		if err != nil {
			if isInitial {
				// For initial connection, wait and retry
				select {
				case <-m.stopCh:
					future.SetFailure(ErrClientStopped)
					m.activeConns.Delete(addr)
					return
				case <-time.After(m.reconnInterval):
					continue
				}
			}
			// For reconnection, wait before retry
			select {
			case <-m.stopCh:
				m.activeConns.Delete(addr)
				return
			case <-time.After(m.reconnDelay):
				continue
			}
		}

		// Connection successful
		if isInitial {
			future.SetConn(conn)
			future.SetSuccess()
			isInitial = false
		}

		// Handle connection based on configuration
		m.handlePersistentConnection(conn)

		// Check if we should reconnect
		select {
		case <-m.stopCh:
			m.activeConns.Delete(addr)
			return
		default:
			m.Logger().Info("Connection lost, reconnecting", "addr", addr)
			select {
			case <-m.stopCh:
				m.activeConns.Delete(addr)
				return
			case <-time.After(m.reconnDelay):
				// Continue reconnect loop
			}
		}
	}
}

func (m *PersistentClientChannelManager) handlePersistentConnection(conn net.Conn) {
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
	if m.ConnHandler != nil {
		m.ConnHandler(conn)
	}

	// Notify disconnection
	m.NotifyDisconnected(conn)
	conn.Close()
}
