package tigerbeetle

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestResolveAddresses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr error
	}{
		{name: "ip is kept", in: []string{"127.0.0.1:3000"}, want: []string{"127.0.0.1:3000"}},
		{name: "ipv6 is kept", in: []string{"[::1]:3000"}, want: []string{"[::1]:3000"}},
		{name: "bare port is kept", in: []string{"3000"}, want: []string{"3000"}},
		{name: "localhost resolves", in: []string{"localhost:3001"}, want: []string{"127.0.0.1:3001"}},
		{name: "mixed", in: []string{"localhost:3000", "10.0.0.2:3001"}, want: []string{"127.0.0.1:3000", "10.0.0.2:3001"}},
		{name: "unknown host", in: []string{"no-such-host.invalid:3000"}, wantErr: ledger.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveAddresses(context.Background(), tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
