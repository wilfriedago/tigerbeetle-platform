// Package config loads the service configuration from environment variables into one typed
// struct and fails fast when a required value is missing.
package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is the complete configuration of the API process. Every field is documented in
// .env.example at the repository root; keep the two in sync.
type Config struct {
	// Addr is the listen address of the HTTP server.
	Addr string `env:"API_ADDR" envDefault:":8080"`
	// LogLevel is one of debug, info, warn, error.
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	// PostgresURL is the pgx connection string.
	PostgresURL string `env:"POSTGRES_URL,required,notEmpty"`
	// TigerBeetleClusterID identifies the cluster; 0 is reserved for development.
	TigerBeetleClusterID uint64 `env:"TIGERBEETLE_CLUSTER_ID" envDefault:"0"`
	// TigerBeetleAddresses lists the replica addresses.
	TigerBeetleAddresses []string `env:"TIGERBEETLE_ADDRESSES,required,notEmpty" envSeparator:","`
	// MigrateOnStart applies pending goose migrations before serving.
	MigrateOnStart bool `env:"MIGRATE_ON_START" envDefault:"false"`
	// ShutdownTimeout bounds the drain of in-flight requests on SIGTERM.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
	// ReadinessTimeout bounds each dependency probe.
	ReadinessTimeout time.Duration `env:"READINESS_TIMEOUT" envDefault:"2s"`
	// TigerBeetleTimeout bounds every TigerBeetle request made for an API call. A write that runs
	// out of time stays pending and the reconciler settles it (ADR-0007).
	TigerBeetleTimeout time.Duration `env:"TIGERBEETLE_TIMEOUT" envDefault:"5s"`
	// ReconcilerGracePeriod is the age after which a pending write is treated as interrupted.
	ReconcilerGracePeriod time.Duration `env:"RECONCILER_GRACE_PERIOD" envDefault:"30s"`
	// ReconcilerInterval is how often the reconciler looks for interrupted writes.
	ReconcilerInterval time.Duration `env:"RECONCILER_INTERVAL" envDefault:"10s"`
	// RESTEnabled serves the REST operations of the ledgers and bookkeeping modules. The health
	// operations are always served (plan section 13.1).
	RESTEnabled bool `env:"REST_ENABLED" envDefault:"true"`
	// GraphQLEnabled serves POST /graphql next to REST (plan section 13.1).
	GraphQLEnabled bool `env:"GRAPHQL_ENABLED" envDefault:"true"`
	// ReconcilerEnabled runs the reconciler in this process (plan section 13.1).
	ReconcilerEnabled bool `env:"RECONCILER_ENABLED" envDefault:"true"`
	// GRPCEnabled serves gRPC on GRPCAddr. Off by default: like the broker transports it stays
	// development-only until authentication is decided (ADR-0021).
	GRPCEnabled bool `env:"GRPC_ENABLED" envDefault:"false"`
	// GRPCAddr is the listen address of the gRPC server.
	GRPCAddr string `env:"GRPC_ADDR" envDefault:":9090"`
	// AMQPRPCEnabled serves the RPC methods over RabbitMQ (ADR-0021); off by default, development
	// only until authentication is decided. It needs AMQPURL.
	AMQPRPCEnabled bool `env:"AMQP_RPC_ENABLED" envDefault:"false"`
	// AMQPURL is the RabbitMQ URL, amqp://user:password@host:5672/vhost.
	AMQPURL string `env:"AMQP_URL"`
	// AMQPRPCQueue is the durable request queue every replica consumes.
	AMQPRPCQueue string `env:"AMQP_RPC_QUEUE" envDefault:"tbp.rpc.v1"`
	// NATSRPCEnabled serves the RPC methods over NATS; off by default, like AMQP RPC. It needs
	// NATSURL.
	NATSRPCEnabled bool `env:"NATS_RPC_ENABLED" envDefault:"false"`
	// NATSURL is the NATS URL, nats://host:4222.
	NATSURL string `env:"NATS_URL"`
	// NATSRPCQueueGroup is the queue group every replica joins, so each request runs once.
	NATSRPCQueueGroup string `env:"NATS_RPC_QUEUE_GROUP" envDefault:"tbp-api"`
	// RPCConcurrency bounds the requests each broker server runs at once.
	RPCConcurrency int `env:"RPC_CONCURRENCY" envDefault:"16"`
	// RPCTimeoutMax bounds every broker request, whatever deadline the client sends.
	RPCTimeoutMax time.Duration `env:"RPC_TIMEOUT_MAX" envDefault:"30s"`
	// KafkaEventsEnabled relays TigerBeetle change events to Kafka (ADR-0022); off by default,
	// development only until authentication is decided. It needs KafkaBrokers and CDCAMQPURL.
	KafkaEventsEnabled bool `env:"KAFKA_EVENTS_ENABLED" envDefault:"false"`
	// KafkaBrokers lists the seed brokers, host:port.
	KafkaBrokers []string `env:"KAFKA_BROKERS" envSeparator:","`
	// KafkaEventsTopic is the topic of ledger events.
	KafkaEventsTopic string `env:"KAFKA_EVENTS_TOPIC" envDefault:"tbp.ledger-events.v1"`
	// CDCAMQPURL is the RabbitMQ the TigerBeetle CDC job publishes to.
	CDCAMQPURL string `env:"CDC_AMQP_URL"`
	// CDCQueue is the relay queue, bound to the CDC exchange tbp.cdc.
	CDCQueue string `env:"CDC_QUEUE" envDefault:"tbp.ledger-events.relay"`

	// Authentication is always on (ADR-0025): there is no switch to turn it off.

	// AuthIssuerURL is the identity provider's issuer, as access tokens name it.
	AuthIssuerURL string `env:"AUTH_ISSUER_URL,required,notEmpty"`
	// AuthDiscoveryURL is where the API reads the provider's discovery document and keys, when it
	// reaches the provider at another address than browsers do. Empty means AUTH_ISSUER_URL.
	AuthDiscoveryURL string `env:"AUTH_DISCOVERY_URL"`
	// AuthAudience is the audience access tokens must carry: the API's identifier.
	AuthAudience string `env:"AUTH_AUDIENCE,required,notEmpty"`
	// AuthStaffClaim and AuthStaffValue mark the platform's staff in a token.
	AuthStaffClaim string `env:"AUTH_STAFF_CLAIM" envDefault:"roles"`
	AuthStaffValue string `env:"AUTH_STAFF_VALUE" envDefault:"tbp-staff"`
	// AuthOrganizationClaim lists the aliases of a person's organizations (ADR-0026).
	AuthOrganizationClaim string `env:"AUTH_ORGANIZATION_CLAIM" envDefault:"organization"`

	// The console's sign-in through the backend-for-frontend (ADR-0029); off when AUTH_CLIENT_ID is
	// empty, for an API that serves no console.

	// AuthClientID and AuthClientSecret are the console's confidential client.
	AuthClientID     string `env:"AUTH_CLIENT_ID"`
	AuthClientSecret string `env:"AUTH_CLIENT_SECRET"`
	// AuthRedirectURL is where the provider sends the browser back, through the console's origin.
	AuthRedirectURL string `env:"AUTH_REDIRECT_URL"`
	// AuthConsoleOrigin is the console's origin, the only one allowed to change state with the
	// session cookie.
	AuthConsoleOrigin string `env:"AUTH_CONSOLE_ORIGIN"`
	// AuthSessionKey seals the stored tokens and the sign-in cookie: 32 bytes in standard base64,
	// comma-separated during a rotation, the new key first.
	AuthSessionKey string `env:"AUTH_SESSION_KEY"`
	// AuthCookieSecure marks the cookies Secure; false only for plain-HTTP development.
	AuthCookieSecure bool `env:"AUTH_COOKIE_SECURE" envDefault:"true"`
}

// Load reads the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Parse reads the given variables instead of the process environment. Tests use it.
func Parse(environ map[string]string) (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environ})
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validate checks cross-field rules and normalises list values in place.
func (c *Config) validate() error {
	if _, err := c.Level(); err != nil {
		return err
	}
	if c.AMQPRPCEnabled && c.AMQPURL == "" {
		return fmt.Errorf("config: AMQP_RPC_ENABLED needs AMQP_URL")
	}
	if c.NATSRPCEnabled && c.NATSURL == "" {
		return fmt.Errorf("config: NATS_RPC_ENABLED needs NATS_URL")
	}
	if c.KafkaEventsEnabled && (len(c.KafkaBrokers) == 0 || c.CDCAMQPURL == "") {
		return fmt.Errorf("config: KAFKA_EVENTS_ENABLED needs KAFKA_BROKERS and CDC_AMQP_URL")
	}
	if (c.AMQPRPCEnabled || c.NATSRPCEnabled) && (c.RPCConcurrency < 1 || c.RPCTimeoutMax <= 0) {
		return fmt.Errorf("config: RPC_CONCURRENCY and RPC_TIMEOUT_MAX must be positive")
	}
	if c.AuthClientID != "" && (c.AuthClientSecret == "" || c.AuthRedirectURL == "" || c.AuthConsoleOrigin == "" || c.AuthSessionKey == "") {
		return fmt.Errorf("config: AUTH_CLIENT_ID needs AUTH_CLIENT_SECRET, AUTH_REDIRECT_URL, AUTH_CONSOLE_ORIGIN and AUTH_SESSION_KEY")
	}
	if c.ShutdownTimeout <= 0 || c.ReadinessTimeout <= 0 || c.TigerBeetleTimeout <= 0 {
		return fmt.Errorf("config: SHUTDOWN_TIMEOUT, READINESS_TIMEOUT and TIGERBEETLE_TIMEOUT must be positive")
	}
	if c.ReconcilerGracePeriod <= c.TigerBeetleTimeout || c.ReconcilerInterval <= 0 {
		return fmt.Errorf("config: RECONCILER_GRACE_PERIOD must exceed TIGERBEETLE_TIMEOUT and RECONCILER_INTERVAL must be positive")
	}
	for i, addr := range c.TigerBeetleAddresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return fmt.Errorf("config: TIGERBEETLE_ADDRESSES contains an empty address")
		}
		c.TigerBeetleAddresses[i] = addr
	}
	return nil
}

// Level converts LogLevel to a slog level.
func (c Config) Level() (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return 0, fmt.Errorf("config: LOG_LEVEL %q is not one of debug, info, warn, error", c.LogLevel)
	}
	return level, nil
}
