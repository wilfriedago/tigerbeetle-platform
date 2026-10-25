//go:build integration

package devstack

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartPostgres runs the pinned Postgres image for the test and returns its connection string.
func StartPostgres(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c, err := tcpostgres.Run(ctx, images.Postgres,
		tcpostgres.WithDatabase("ledger"),
		tcpostgres.WithUsername("ledger"),
		tcpostgres.WithPassword("ledger"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres %s: %v", images.Postgres, err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres: %v", err)
		}
	})
	url, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return url
}

// StartTigerBeetle runs a single-replica cluster of the pinned image and returns its address as
// IP:port. The data file is formatted inside the same container right before start, mirroring
// deploy/compose/compose.yaml.
func StartTigerBeetle(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	images, err := Load()
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
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
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
