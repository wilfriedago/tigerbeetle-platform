//go:build integration

package tigerbeetle

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// startTigerBeetle runs a single-replica cluster. The data file is formatted inside the same
// container right before start, mirroring deploy/compose/compose.yaml.
func startTigerBeetle(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	images, err := devstack.Load()
	if err != nil {
		t.Fatal(err)
	}
	req := testcontainers.ContainerRequest{
		Image:      images.TigerBeetle,
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd: []string{
			"mkdir -p /data && /tigerbeetle format --cluster=0 --replica=0 --replica-count=1 /data/0_0.tigerbeetle && " +
				"exec /tigerbeetle start --addresses=0.0.0.0:3000 --cache-grid=256MiB /data/0_0.tigerbeetle",
		},
		ExposedPorts: []string{"3000/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			// Same options as compose.yaml: io_uring needs the seccomp override on Docker 25+.
			hc.SecurityOpt = append(hc.SecurityOpt, "seccomp=unconfined")
			hc.CapAdd = append(hc.CapAdd, "IPC_LOCK")
		},
		WaitingFor: wait.ForLog("listening on").WithStartupTimeout(2 * time.Minute),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start tigerbeetle %s: %v", images.TigerBeetle, err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("terminate tigerbeetle: %v", err)
		}
	})
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := c.MappedPort(ctx, "3000/tcp")
	if err != nil {
		t.Fatal(err)
	}
	// The TigerBeetle client accepts IP addresses only, not host names such as "localhost".
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		t.Fatalf("resolve docker host %q: %v", host, err)
	}
	ip := ips[0].IP
	for _, candidate := range ips {
		if v4 := candidate.IP.To4(); v4 != nil {
			ip = v4
			break
		}
	}
	return net.JoinHostPort(ip.String(), port.Port())
}

func TestClientIntegration(t *testing.T) {
	address := startTigerBeetle(t)

	client, err := New(0, []string{address})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(client.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	// Concurrent probes share one in-flight request and all succeed.
	errs := make(chan error, 5)
	for range 5 {
		go func() { errs <- client.Ready(ctx) }()
	}
	for range 5 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent Ready: %v", err)
		}
	}

	id := client.NewID()
	if id.IsZero() {
		t.Fatal("NewID returned zero")
	}
}

func TestClientUnreachable(t *testing.T) {
	client, err := New(0, []string{"127.0.0.1:1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(client.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err = client.Ready(ctx)
	if !errors.Is(err, ledger.ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Ready error = %v, want ErrUnavailable wrapping DeadlineExceeded", err)
	}
}
