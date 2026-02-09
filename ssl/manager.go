package ssl

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

var (
	manager     = &SslManager{configs: make(map[string]*tls.Config)}
	managerOnce sync.Once
)

// Manager returns the global SSL manager instance.
func Manager() *SslManager {
	return manager
}

// SslManager manages SSL/TLS contexts and configurations.
type SslManager struct {
	configs map[string]*tls.Config
	mu      sync.RWMutex
	logger  *slog.Logger
}

func init() {
	manager.logger = slog.Default().With("component", "SslManager")
}

// LoadKeyStore loads a certificate and key, initializes a TLS config, and caches it.
// For server: provide both certFile and keyFile
// For client (trust store): provide only certFile as CA certificate
func (m *SslManager) LoadKeyStore(id string, certFile, keyFile string) error {
	m.logger.Info("Loading keystore", "id", id, "cert", certFile)

	var config *tls.Config

	if keyFile != "" {
		// Server certificate with private key
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load key pair: %w", err)
		}

		config = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	} else {
		// CA certificate for client trust store
		caCert, err := os.ReadFile(certFile)
		if err != nil {
			return fmt.Errorf("failed to read CA cert: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA cert")
		}

		config = &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		}
	}

	m.mu.Lock()
	m.configs[id] = config
	m.mu.Unlock()

	return nil
}

// LoadPKCS12 loads a PKCS12 file (not directly supported in Go stdlib).
// Use LoadKeyStore with PEM files instead, or implement PKCS12 parsing.
func (m *SslManager) LoadPKCS12(id, file, password, keyPassword string) error {
	return fmt.Errorf("PKCS12 not directly supported; convert to PEM format first")
}

// GetConfig returns the TLS config for the given ID.
func (m *SslManager) GetConfig(id string) *tls.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[id]
}

// CreateServerFactory returns a server TLS config factory.
func (m *SslManager) CreateServerFactory(id string) TLSConfigFactory {
	config := m.GetConfig(id)
	if config == nil {
		return nil
	}
	return NewServerTLSConfigFactory(config)
}

// CreateClientFactory returns a client TLS config factory.
func (m *SslManager) CreateClientFactory(id string) TLSConfigFactory {
	config := m.GetConfig(id)
	if config == nil {
		return nil
	}
	return NewClientTLSConfigFactory(config)
}

// CreateClientFactoryWithServer returns a client TLS config factory with server name.
func (m *SslManager) CreateClientFactoryWithServer(id, serverName string) TLSConfigFactory {
	config := m.GetConfig(id)
	if config == nil {
		return nil
	}
	return NewClientTLSConfigFactoryWithServer(config, serverName)
}
