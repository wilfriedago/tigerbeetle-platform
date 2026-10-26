// Package tlsconfig builds the TLS configurations of the API's own listeners and of its
// connections to the brokers (plan section 10, phase 7), from PEM files named in the
// configuration. A certificate is read at start-up; rotating it means restarting the process.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// Server returns the configuration of a listener that presents the certificate, or nil when
// both files are empty, for a listener that serves plaintext behind a TLS-terminating proxy.
func Server(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" && keyFile == "" {
		return nil, nil //nolint:nilnil // no TLS is a valid answer
	}
	if certFile == "" || keyFile == "" {
		return nil, errors.New("tlsconfig: a certificate needs both its file and its key file")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tlsconfig: certificate: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
}

// Client returns the configuration of a connection that trusts the CAs in caFile, or the
// system's when caFile is empty.
func Client(caFile string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return cfg, nil
	}
	pem, err := os.ReadFile(caFile) //nolint:gosec // the path comes from the operator's configuration
	if err != nil {
		return nil, fmt.Errorf("tlsconfig: CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("tlsconfig: no certificate in %s", caFile)
	}
	cfg.RootCAs = pool
	return cfg, nil
}

// ClientWhen is Client when enabled or a CA file is given, nil otherwise: for a connection that
// turns TLS on only when asked.
func ClientWhen(enabled bool, caFile string) (*tls.Config, error) {
	if !enabled && caFile == "" {
		return nil, nil //nolint:nilnil // no TLS is a valid answer
	}
	return Client(caFile)
}
