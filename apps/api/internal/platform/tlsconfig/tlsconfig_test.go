package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// selfSigned writes a self-signed certificate and its key, and returns their paths.
func selfSigned(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "api"}, DNSNames: []string{"api"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func TestServer(t *testing.T) {
	certFile, keyFile := selfSigned(t)
	cfg, err := Server(certFile, keyFile)
	if err != nil || cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("Server = %v, %v", cfg, err)
	}
	if cfg, err := Server("", ""); cfg != nil || err != nil {
		t.Errorf("Server without files = %v, %v; want plaintext", cfg, err)
	}
	if _, err := Server(certFile, ""); err == nil {
		t.Error("a certificate without its key was accepted")
	}
}

func TestClient(t *testing.T) {
	certFile, _ := selfSigned(t)
	cfg, err := Client(certFile)
	if err != nil || cfg.RootCAs == nil {
		t.Fatalf("Client = %v, %v", cfg, err)
	}
	if cfg, err := ClientWhen(false, ""); cfg != nil || err != nil {
		t.Errorf("ClientWhen off = %v, %v; want no TLS", cfg, err)
	}
	if cfg, err := ClientWhen(true, ""); err != nil || cfg == nil || cfg.RootCAs != nil {
		t.Errorf("ClientWhen on = %v, %v; want the system roots", cfg, err)
	}
	if _, err := Client(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Error("a missing CA file was accepted")
	}
}
