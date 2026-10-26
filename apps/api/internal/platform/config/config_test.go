package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	t.Parallel()
	required := map[string]string{
		"POSTGRES_URL":          "postgres://u:p@localhost:5432/db",
		"TIGERBEETLE_ADDRESSES": "localhost:3000,localhost:3001",
		"AUTH_ISSUER_URL":       "http://localhost:18080/realms/tbp",
		"AUTH_AUDIENCE":         "tbp-api",
	}
	tests := []struct {
		name    string
		environ map[string]string
		want    Config
		wantErr bool
	}{
		{
			name:    "defaults",
			environ: required,
			want: Config{
				Addr:                  ":8080",
				LogLevel:              "info",
				PostgresURL:           "postgres://u:p@localhost:5432/db",
				TigerBeetleClusterID:  0,
				TigerBeetleAddresses:  []string{"localhost:3000", "localhost:3001"},
				ShutdownTimeout:       10 * time.Second,
				ReadinessTimeout:      2 * time.Second,
				TigerBeetleTimeout:    5 * time.Second,
				ReconcilerGracePeriod: 30 * time.Second,
				ReconcilerInterval:    10 * time.Second,
				RESTEnabled:           true,
				GraphQLEnabled:        true,
				ReconcilerEnabled:     true,
				GRPCAddr:              ":9090",
				AMQPRPCQueue:          "tbp.rpc.v1",
				NATSRPCQueueGroup:     "tbp-api",
				RPCConcurrency:        16,
				RPCTimeoutMax:         30 * time.Second,
				KafkaEventsTopic:      "tbp.ledger-events.v1",
				CDCQueue:              "tbp.ledger-events.relay",
				AuthIssuerURL:         "http://localhost:18080/realms/tbp",
				AuthAudience:          "tbp-api",
				AuthStaffClaim:        "roles",
				AuthStaffValue:        "tbp-staff",
				AuthOrganizationClaim: "organization",
			},
		},
		{
			name: "overrides",
			environ: merge(required, map[string]string{
				"API_ADDR": ":9000", "LOG_LEVEL": "debug", "TIGERBEETLE_CLUSTER_ID": "7",
				"MIGRATE_ON_START": "true", "SHUTDOWN_TIMEOUT": "1s", "READINESS_TIMEOUT": "500ms",
				"RECONCILER_ENABLED": "false", "GRPC_ENABLED": "true", "GRPC_ADDR": ":9191",
			}),
			want: Config{
				Addr: ":9000", LogLevel: "debug", PostgresURL: "postgres://u:p@localhost:5432/db",
				TigerBeetleClusterID: 7, TigerBeetleAddresses: []string{"localhost:3000", "localhost:3001"},
				MigrateOnStart: true, ShutdownTimeout: time.Second, ReadinessTimeout: 500 * time.Millisecond,
				TigerBeetleTimeout: 5 * time.Second, ReconcilerGracePeriod: 30 * time.Second, ReconcilerInterval: 10 * time.Second,
				RESTEnabled: true, GraphQLEnabled: true, GRPCEnabled: true, GRPCAddr: ":9191",
				AMQPRPCQueue: "tbp.rpc.v1", NATSRPCQueueGroup: "tbp-api", RPCConcurrency: 16, RPCTimeoutMax: 30 * time.Second,
				KafkaEventsTopic: "tbp.ledger-events.v1", CDCQueue: "tbp.ledger-events.relay",
				AuthIssuerURL: "http://localhost:18080/realms/tbp", AuthAudience: "tbp-api",
				AuthStaffClaim: "roles", AuthStaffValue: "tbp-staff", AuthOrganizationClaim: "organization",
			},
		},
		{name: "missing issuer", environ: map[string]string{"POSTGRES_URL": "postgres://x", "TIGERBEETLE_ADDRESSES": "localhost:3000", "AUTH_AUDIENCE": "tbp-api"}, wantErr: true},
		{name: "missing postgres url", environ: map[string]string{"TIGERBEETLE_ADDRESSES": "localhost:3000"}, wantErr: true},
		{name: "missing tigerbeetle addresses", environ: map[string]string{"POSTGRES_URL": "postgres://x"}, wantErr: true},
		{name: "bad log level", environ: merge(required, map[string]string{"LOG_LEVEL": "loud"}), wantErr: true},
		{name: "bad duration", environ: merge(required, map[string]string{"SHUTDOWN_TIMEOUT": "soon"}), wantErr: true},
		{name: "zero timeout", environ: merge(required, map[string]string{"READINESS_TIMEOUT": "0s"}), wantErr: true},
		{name: "grace shorter than timeout", environ: merge(required, map[string]string{"RECONCILER_GRACE_PERIOD": "2s"}), wantErr: true},
		{name: "amqp rpc without url", environ: merge(required, map[string]string{"AMQP_RPC_ENABLED": "true"}), wantErr: true},
		{name: "nats rpc without url", environ: merge(required, map[string]string{"NATS_RPC_ENABLED": "true"}), wantErr: true},
		{name: "kafka events without brokers", environ: merge(required, map[string]string{"KAFKA_EVENTS_ENABLED": "true", "CDC_AMQP_URL": "amqp://x"}), wantErr: true},
		{name: "kafka events without cdc url", environ: merge(required, map[string]string{"KAFKA_EVENTS_ENABLED": "true", "KAFKA_BROKERS": "k:9092"}), wantErr: true},
		{name: "zero rpc concurrency", environ: merge(required, map[string]string{"NATS_RPC_ENABLED": "true", "NATS_URL": "nats://x:4222", "RPC_CONCURRENCY": "0"}), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.environ)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Parse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLevel(t *testing.T) {
	t.Parallel()
	level, err := Config{LogLevel: "warn"}.Level()
	if err != nil || level != slog.LevelWarn {
		t.Fatalf("Level() = %v, %v", level, err)
	}
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
