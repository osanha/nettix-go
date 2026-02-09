package channel

import (
	"net"
	"time"
)

// SocketOptions contains socket configuration options.
type SocketOptions struct {
	RecvBufferSize int
	SendBufferSize int
	SoLinger       int  // -1 means disabled
	KeepAlive      bool
	TcpNoDelay     bool
	ReuseAddress   bool
}

// DefaultSocketOptions returns default socket options.
func DefaultSocketOptions() SocketOptions {
	return SocketOptions{
		RecvBufferSize: 0, // System default
		SendBufferSize: 0, // System default
		SoLinger:       -1,
		KeepAlive:      false,
		TcpNoDelay:     true,
		ReuseAddress:   true,
	}
}

// Apply applies socket options to a TCP connection.
func (o *SocketOptions) Apply(conn *net.TCPConn) error {
	if o.RecvBufferSize > 0 {
		if err := conn.SetReadBuffer(o.RecvBufferSize); err != nil {
			return err
		}
	}
	if o.SendBufferSize > 0 {
		if err := conn.SetWriteBuffer(o.SendBufferSize); err != nil {
			return err
		}
	}
	if o.SoLinger >= 0 {
		if err := conn.SetLinger(o.SoLinger); err != nil {
			return err
		}
	}
	if err := conn.SetKeepAlive(o.KeepAlive); err != nil {
		return err
	}
	if err := conn.SetNoDelay(o.TcpNoDelay); err != nil {
		return err
	}
	return nil
}

// ServerOptions contains server-specific socket options.
type ServerOptions struct {
	SocketOptions
	Backlog int
}

// DefaultServerOptions returns default server options.
func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		SocketOptions: DefaultSocketOptions(),
		Backlog:       1024,
	}
}

// ClientOptions contains client-specific socket options.
type ClientOptions struct {
	SocketOptions
	ConnectTimeout time.Duration
}

// DefaultClientOptions returns default client options.
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		SocketOptions:  DefaultSocketOptions(),
		ConnectTimeout: 30 * time.Second,
	}
}
