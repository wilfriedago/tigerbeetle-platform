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
}

// Load reads the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, cfg.validate()
}

// Parse reads the given variables instead of the process environment. Tests use it.
func Parse(environ map[string]string) (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environ})
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if _, err := c.Level(); err != nil {
		return err
	}
	if c.ShutdownTimeout <= 0 || c.ReadinessTimeout <= 0 {
		return fmt.Errorf("config: SHUTDOWN_TIMEOUT and READINESS_TIMEOUT must be positive")
	}
	for _, addr := range c.TigerBeetleAddresses {
		if strings.TrimSpace(addr) == "" {
			return fmt.Errorf("config: TIGERBEETLE_ADDRESSES contains an empty address")
		}
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
