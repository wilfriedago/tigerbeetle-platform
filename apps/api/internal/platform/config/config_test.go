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
				ReconcilerEnabled:     true,
			},
		},
		{
			name: "overrides",
			environ: merge(required, map[string]string{
				"API_ADDR": ":9000", "LOG_LEVEL": "debug", "TIGERBEETLE_CLUSTER_ID": "7",
				"MIGRATE_ON_START": "true", "SHUTDOWN_TIMEOUT": "1s", "READINESS_TIMEOUT": "500ms",
				"RECONCILER_ENABLED": "false",
			}),
			want: Config{
				Addr: ":9000", LogLevel: "debug", PostgresURL: "postgres://u:p@localhost:5432/db",
				TigerBeetleClusterID: 7, TigerBeetleAddresses: []string{"localhost:3000", "localhost:3001"},
				MigrateOnStart: true, ShutdownTimeout: time.Second, ReadinessTimeout: 500 * time.Millisecond,
				TigerBeetleTimeout: 5 * time.Second, ReconcilerGracePeriod: 30 * time.Second, ReconcilerInterval: 10 * time.Second,
			},
		},
		{name: "missing postgres url", environ: map[string]string{"TIGERBEETLE_ADDRESSES": "localhost:3000"}, wantErr: true},
		{name: "missing tigerbeetle addresses", environ: map[string]string{"POSTGRES_URL": "postgres://x"}, wantErr: true},
		{name: "bad log level", environ: merge(required, map[string]string{"LOG_LEVEL": "loud"}), wantErr: true},
		{name: "bad duration", environ: merge(required, map[string]string{"SHUTDOWN_TIMEOUT": "soon"}), wantErr: true},
		{name: "zero timeout", environ: merge(required, map[string]string{"READINESS_TIMEOUT": "0s"}), wantErr: true},
		{name: "grace shorter than timeout", environ: merge(required, map[string]string{"RECONCILER_GRACE_PERIOD": "2s"}), wantErr: true},
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
