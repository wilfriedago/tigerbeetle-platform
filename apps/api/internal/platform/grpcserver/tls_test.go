package grpcserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log/slog"
	"math/big"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn/authntest"
)

// TestTLS serves with a certificate (GRPC_TLS_CERT_FILE): a client that trusts it gets an answer,
// a plaintext client does not.
func TestTLS(t *testing.T) {
	t.Parallel()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "api"}, DNSNames: []string{"api"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Options{
		ShutdownTimeout: time.Second, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: authntest.Default(),
		TLS: &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12},
	})
	probe := &probeService{release: make(chan struct{})}
	srv.RegisterService(probe.desc(), probe)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: roots, ServerName: "api", MinVersion: tls.VersionTLS12})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.Invoke(t.Context(), "/probe.Probe/RequestID", new(emptypb.Empty), new(wrapperspb.StringValue)); err != nil {
		t.Fatalf("over TLS: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	plain := dialAddr(t, lis.Addr().String())
	if err := plain.Invoke(ctx, "/probe.Probe/RequestID", new(emptypb.Empty), new(wrapperspb.StringValue)); err == nil {
		t.Fatal("a plaintext client got an answer from a TLS server")
	}
}
