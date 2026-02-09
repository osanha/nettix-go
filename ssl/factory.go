package ssl

import (
	"crypto/tls"
)

// TLSConfigFactory is an interface for creating TLS configurations.
type TLSConfigFactory interface {
	// CreateTLSConfig creates a new TLS configuration.
	CreateTLSConfig() *tls.Config
}

// ClientTLSConfigFactory creates TLS configurations for client connections.
type ClientTLSConfigFactory struct {
	config     *tls.Config
	serverName string
}

// NewClientTLSConfigFactory creates a new client TLS config factory.
func NewClientTLSConfigFactory(config *tls.Config) *ClientTLSConfigFactory {
	return &ClientTLSConfigFactory{config: config}
}

// NewClientTLSConfigFactoryWithServer creates a factory with server name for session reuse.
func NewClientTLSConfigFactoryWithServer(config *tls.Config, serverName string) *ClientTLSConfigFactory {
	return &ClientTLSConfigFactory{
		config:     config,
		serverName: serverName,
	}
}

// CreateTLSConfig creates a new TLS configuration for client mode.
func (f *ClientTLSConfigFactory) CreateTLSConfig() *tls.Config {
	cfg := f.config.Clone()
	if f.serverName != "" {
		cfg.ServerName = f.serverName
	}
	return cfg
}

// ServerTLSConfigFactory creates TLS configurations for server connections.
type ServerTLSConfigFactory struct {
	config *tls.Config
}

// NewServerTLSConfigFactory creates a new server TLS config factory.
func NewServerTLSConfigFactory(config *tls.Config) *ServerTLSConfigFactory {
	return &ServerTLSConfigFactory{config: config}
}

// CreateTLSConfig creates a new TLS configuration for server mode.
func (f *ServerTLSConfigFactory) CreateTLSConfig() *tls.Config {
	return f.config.Clone()
}
