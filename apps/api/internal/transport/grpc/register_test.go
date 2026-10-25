package grpc

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	grpclib "google.golang.org/grpc"
)

// TestRegisterAll lists every service and method the server answers; a method added to a proto
// without a registration, or the reverse, shows here.
func TestRegisterAll(t *testing.T) {
	t.Parallel()
	srv := grpclib.NewServer()
	RegisterAll(srv, Modules{})
	got := map[string][]string{}
	for name, info := range srv.GetServiceInfo() {
		for _, m := range info.Methods {
			got[name] = append(got[name], m.Name)
		}
		slices.Sort(got[name])
	}
	want := map[string][]string{
		"tigerbeetle.platform.v1.LedgerService": {
			"CreateLedger", "GetLedger", "GetLedgerStats", "ListAssets", "ListLedgers", "ReplaceLedgerMetadata", "Search",
		},
		"tigerbeetle.platform.v1.SettingsService": {
			"CreateEventType", "DeleteRule", "ListEventTypes", "ListRules", "MatchRule", "PutRule",
		},
		"tigerbeetle.platform.v1.AccountService": {
			"AggregateBalances", "GetAccount", "GetAccountHistory", "ListAccounts", "ListSegments", "ReplaceAccountMetadata",
		},
		"tigerbeetle.platform.v1.TransactionService": {
			"CreateTransaction", "GetTransaction", "ListTransactions", "ReplaceTransactionMetadata", "RevertTransaction",
		},
		"tigerbeetle.platform.v1.HoldService": {
			"CaptureHold", "CreateHold", "GetHold", "ListHolds", "VoidHold",
		},
		"tigerbeetle.platform.v1.WriteRequestService": {"GetWriteRequest", "ListWriteRequests"},
		"tigerbeetle.platform.v1.OperationsService":   {"GetInfo", "GetReconcilerStatus"},
		"grpc.health.v1.Health":                       {"Check", "List", "Watch"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("registered services (-want +got):\n%s", diff)
	}
	methods := 0
	for name, m := range got {
		if name != "grpc.health.v1.Health" {
			methods += len(m)
		}
	}
	// Every REST operation but liveness and readiness, which grpc.health.v1 covers.
	if methods != 33 {
		t.Errorf("%d platform methods, want 33: the 35 REST operations less liveness and readiness", methods)
	}
}
