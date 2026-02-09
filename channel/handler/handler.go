package handler

import "net"

// Handler is the interface for handling channel events.
type Handler interface {
	// OnConnected is called when a connection is established.
	OnConnected(conn net.Conn) error

	// OnDisconnected is called when a connection is closed.
	OnDisconnected(conn net.Conn) error

	// OnMessage is called when a message is received.
	OnMessage(conn net.Conn, msg interface{}) error

	// OnError is called when an error occurs.
	OnError(conn net.Conn, err error)
}

// BaseHandler provides a default implementation of Handler.
type BaseHandler struct{}

func (h *BaseHandler) OnConnected(conn net.Conn) error              { return nil }
func (h *BaseHandler) OnDisconnected(conn net.Conn) error           { return nil }
func (h *BaseHandler) OnMessage(conn net.Conn, msg interface{}) error { return nil }
func (h *BaseHandler) OnError(conn net.Conn, err error)             {}

// InboundMessageHandler handles only inbound messages.
type InboundMessageHandler interface {
	// OnMessageReceived handles a received message.
	OnMessageReceived(conn net.Conn, msg interface{}) error
}

// OutboundMessageHandler handles only outbound messages.
type OutboundMessageHandler interface {
	// OnMessageSent handles a message about to be sent.
	OnMessageSent(conn net.Conn, msg interface{}) error
}

// MessageHandler handles both inbound and outbound messages.
type MessageHandler interface {
	InboundMessageHandler
	OutboundMessageHandler
}

// ConnectStateHandler handles only connection state events.
type ConnectStateHandler struct {
	OnConnectedFunc    func(conn net.Conn) error
	OnDisconnectedFunc func(conn net.Conn) error
}

// OnConnected handles the connected event.
func (h *ConnectStateHandler) OnConnected(conn net.Conn) error {
	if h.OnConnectedFunc != nil {
		return h.OnConnectedFunc(conn)
	}
	return nil
}

// OnDisconnected handles the disconnected event.
func (h *ConnectStateHandler) OnDisconnected(conn net.Conn) error {
	if h.OnDisconnectedFunc != nil {
		return h.OnDisconnectedFunc(conn)
	}
	return nil
}

func (h *ConnectStateHandler) OnMessage(conn net.Conn, msg interface{}) error { return nil }
func (h *ConnectStateHandler) OnError(conn net.Conn, err error)               {}
