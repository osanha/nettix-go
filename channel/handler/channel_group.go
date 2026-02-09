package handler

import (
	"net"
	"sync"
	"sync/atomic"
)

// ChannelGroup manages a group of connections.
type ChannelGroup struct {
	name        string
	connections sync.Map // map[net.Conn]struct{}
	count       atomic.Int32
}

// NewChannelGroup creates a new channel group.
func NewChannelGroup(name string) *ChannelGroup {
	return &ChannelGroup{
		name: name,
	}
}

// Add adds a connection to the group.
func (g *ChannelGroup) Add(conn net.Conn) {
	if _, loaded := g.connections.LoadOrStore(conn, struct{}{}); !loaded {
		g.count.Add(1)
	}
}

// Remove removes a connection from the group.
func (g *ChannelGroup) Remove(conn net.Conn) {
	if _, ok := g.connections.LoadAndDelete(conn); ok {
		g.count.Add(-1)
	}
}

// Contains returns true if the connection is in the group.
func (g *ChannelGroup) Contains(conn net.Conn) bool {
	_, ok := g.connections.Load(conn)
	return ok
}

// Size returns the number of connections in the group.
func (g *ChannelGroup) Size() int {
	return int(g.count.Load())
}

// Name returns the name of the group.
func (g *ChannelGroup) Name() string {
	return g.name
}

// ForEach iterates over all connections in the group.
func (g *ChannelGroup) ForEach(fn func(conn net.Conn) bool) {
	g.connections.Range(func(key, value interface{}) bool {
		return fn(key.(net.Conn))
	})
}

// Close closes all connections in the group.
func (g *ChannelGroup) Close() error {
	var lastErr error
	g.connections.Range(func(key, value interface{}) bool {
		conn := key.(net.Conn)
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		g.connections.Delete(key)
		return true
	})
	g.count.Store(0)
	return lastErr
}

// Broadcast sends data to all connections in the group.
func (g *ChannelGroup) Broadcast(data []byte) error {
	var lastErr error
	g.connections.Range(func(key, value interface{}) bool {
		conn := key.(net.Conn)
		if _, err := conn.Write(data); err != nil {
			lastErr = err
		}
		return true
	})
	return lastErr
}

// ChannelGroupHandler is a handler that manages a group of connections.
type ChannelGroupHandler struct {
	group *ChannelGroup
}

// NewChannelGroupHandler creates a new channel group handler.
func NewChannelGroupHandler() *ChannelGroupHandler {
	return &ChannelGroupHandler{
		group: NewChannelGroup(""),
	}
}

// NewChannelGroupHandlerWithName creates a new named channel group handler.
func NewChannelGroupHandlerWithName(name string) *ChannelGroupHandler {
	return &ChannelGroupHandler{
		group: NewChannelGroup(name),
	}
}

// OnConnected adds the connection to the group.
func (h *ChannelGroupHandler) OnConnected(conn net.Conn) error {
	h.group.Add(conn)
	return nil
}

// OnDisconnected removes the connection from the group.
func (h *ChannelGroupHandler) OnDisconnected(conn net.Conn) error {
	h.group.Remove(conn)
	return nil
}

// OnMessage is a no-op for this handler.
func (h *ChannelGroupHandler) OnMessage(conn net.Conn, msg interface{}) error {
	return nil
}

// OnError is a no-op for this handler.
func (h *ChannelGroupHandler) OnError(conn net.Conn, err error) {}

// Connections returns the channel group.
func (h *ChannelGroupHandler) Connections() *ChannelGroup {
	return h.group
}
