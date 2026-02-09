package ssl

import (
	"crypto/tls"
	"log/slog"
	"net"
	"time"
)

// Handshaker handles the TLS handshake process.
type Handshaker struct {
	logger  *slog.Logger
	timeout time.Duration
}

// NewHandshaker creates a new Handshaker.
func NewHandshaker() *Handshaker {
	return &Handshaker{
		logger:  slog.Default().With("component", "SslHandshaker"),
		timeout: 30 * time.Second,
	}
}

// NewHandshakerWithName creates a new Handshaker with a custom logger name.
func NewHandshakerWithName(name string) *Handshaker {
	return &Handshaker{
		logger:  slog.Default().With("component", "SslHandshaker."+name),
		timeout: 30 * time.Second,
	}
}

// SetTimeout sets the handshake timeout.
func (h *Handshaker) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

// HandshakeServer performs a TLS handshake as server.
func (h *Handshaker) HandshakeServer(conn net.Conn, config *tls.Config) (*tls.Conn, error) {
	tlsConn := tls.Server(conn, config)
	return h.doHandshake(tlsConn)
}

// HandshakeClient performs a TLS handshake as client.
func (h *Handshaker) HandshakeClient(conn net.Conn, config *tls.Config) (*tls.Conn, error) {
	tlsConn := tls.Client(conn, config)
	return h.doHandshake(tlsConn)
}

func (h *Handshaker) doHandshake(tlsConn *tls.Conn) (*tls.Conn, error) {
	// Set handshake deadline
	if err := tlsConn.SetDeadline(time.Now().Add(h.timeout)); err != nil {
		tlsConn.Close()
		return nil, err
	}

	// Perform handshake
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, err
	}

	// Clear deadline
	if err := tlsConn.SetDeadline(time.Time{}); err != nil {
		tlsConn.Close()
		return nil, err
	}

	state := tlsConn.ConnectionState()
	h.logger.Info("SSL session established",
		"remote", tlsConn.RemoteAddr().String(),
		"cipherSuite", tls.CipherSuiteName(state.CipherSuite),
		"version", tlsVersionName(state.Version))

	return tlsConn, nil
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "Unknown"
	}
}
