//go:build integration

package devstack

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
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

// StartRabbitMQ runs the pinned RabbitMQ image for the test and returns its AMQP URL.
func StartRabbitMQ(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c, err := tcrabbitmq.Run(ctx, images.RabbitMQ)
	if err != nil {
		t.Fatalf("start rabbitmq %s: %v", images.RabbitMQ, err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("terminate rabbitmq: %v", err)
		}
	})
	url, err := c.AmqpURL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return url
}

// StartNATS runs the pinned NATS image for the test and returns its URL. JetStream stays off, as
// in deploy/compose/compose.yaml.
func StartNATS(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c, err := tcnats.Run(ctx, images.NATS, testcontainers.WithCmd("-m", "8222"))
	if err != nil {
		t.Fatalf("start nats %s: %v", images.NATS, err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("terminate nats: %v", err)
		}
	})
	url, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return url
}

// StartKafka runs the pinned Kafka image as one KRaft node and returns its address for clients on
// the host. Kafka hands clients the address it advertises, so the host port is chosen first and
// advertised as is, instead of a random mapped port the broker could not know.
func StartKafka(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	req := testcontainers.ContainerRequest{
		Image:        images.Kafka,
		ExposedPorts: []string{"9092/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				network.MustParsePort("9092/tcp"): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: port}},
			}
		},
		Env: map[string]string{
			"KAFKA_NODE_ID":                                  "1",
			"KAFKA_PROCESS_ROLES":                            "broker,controller",
			"KAFKA_LISTENERS":                                "PLAINTEXT://:9092,CONTROLLER://:9093",
			"KAFKA_ADVERTISED_LISTENERS":                     "PLAINTEXT://127.0.0.1:" + port,
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":           "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_CONTROLLER_LISTENER_NAMES":                "CONTROLLER",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":                 "1@localhost:9093",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
			"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS":         "0",
			"KAFKA_NUM_PARTITIONS":                           "3",
		},
		WaitingFor: wait.ForLog("Kafka Server started").WithStartupTimeout(2 * time.Minute),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start kafka %s: %v", images.Kafka, err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("terminate kafka: %v", err)
		}
	})
	return "127.0.0.1:" + port
}

// freePort asks the kernel for a free TCP port on the host.
func freePort(t testing.TB) string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}
