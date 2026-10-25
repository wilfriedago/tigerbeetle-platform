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
	host, _ := StartTigerBeetleNode(t)
	return host
}

// StartTigerBeetleNode is StartTigerBeetle that also returns the address other containers reach
// the replica at, for the CDC job, which accepts IP addresses only.
func StartTigerBeetleNode(t testing.TB) (host, inside string) {
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
	dockerHost, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := c.MappedPort(ctx, "3000/tcp")
	if err != nil {
		t.Fatal(err)
	}
	// The TigerBeetle client accepts IP addresses only, not host names such as "localhost".
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, dockerHost)
	if err != nil || len(ips) == 0 {
		t.Fatalf("resolve docker host %q: %v", dockerHost, err)
	}
	ip := ips[0].IP
	for _, candidate := range ips {
		if v4 := candidate.IP.To4(); v4 != nil {
			ip = v4
			break
		}
	}
	containerIP, err := c.ContainerIP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return net.JoinHostPort(ip.String(), port.Port()), net.JoinHostPort(containerIP, "3000")
}

// StartRabbitMQ runs the pinned RabbitMQ image for the test and returns its AMQP URL.
func StartRabbitMQ(t testing.TB) string {
	t.Helper()
	url, _ := StartRabbitMQNode(t)
	return url
}

// StartRabbitMQNode is StartRabbitMQ that also returns the address other containers reach the
// broker at, IP:5672, for the CDC job.
func StartRabbitMQNode(t testing.TB) (url, inside string) {
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
	amqpURL, err := c.AmqpURL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	containerIP, err := c.ContainerIP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return amqpURL, net.JoinHostPort(containerIP, "5672")
}

// CDC is a running TigerBeetle CDC job.
type CDC struct {
	container testcontainers.Container
}

// Stop kills the job, as a crash would.
func (c *CDC) Stop(t testing.TB) {
	t.Helper()
	timeout := time.Duration(0)
	if err := c.container.Stop(context.Background(), &timeout); err != nil {
		t.Fatal(err)
	}
}

// Start starts the job again; it resumes after the last timestamp it published.
func (c *CDC) Start(t testing.TB) {
	t.Helper()
	if err := c.container.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// StartCDC runs the TigerBeetle CDC job from the pinned server image, as compose does
// (ADR-0022), publishing the cluster's committed transfers to exchange on RabbitMQ. Both
// addresses are IP:port on the container network; the job resolves no names.
func StartCDC(t testing.TB, tigerbeetle, rabbitmq, exchange string) *CDC {
	t.Helper()
	ctx := context.Background()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	req := testcontainers.ContainerRequest{
		Image: images.TigerBeetle,
		Cmd: []string{
			"amqp", "--cluster=0", "--addresses=" + tigerbeetle, "--host=" + rabbitmq,
			"--vhost=/", "--user=guest", "--password=guest", "--publish-exchange=" + exchange,
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.SecurityOpt = append(hc.SecurityOpt, "seccomp=unconfined")
		},
		WaitingFor: wait.ForLog("Starting CDC").WithStartupTimeout(time.Minute),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start cdc job: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("terminate cdc job: %v", err)
		}
	})
	return &CDC{container: c}
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
