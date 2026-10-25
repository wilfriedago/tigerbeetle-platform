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
