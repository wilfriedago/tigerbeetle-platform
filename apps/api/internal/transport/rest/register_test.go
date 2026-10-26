package rest

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// registered lists the operations of the committed spec, by tag, as the generated spec must
// declare them.
var registered = []struct {
	id, method, path, tag string
}{
	{"getLiveness", http.MethodGet, "/healthz", "operations"},
	{"getReadiness", http.MethodGet, "/readyz", "operations"},
	{"getInfo", http.MethodGet, "/v1/_info", "operations"},
	{"getReconcilerStatus", http.MethodGet, "/v1/_reconciler", "operations"},
	{"listLedgers", http.MethodGet, "/v1/ledgers", "ledgers"},
	{"getLedger", http.MethodGet, "/v1/ledgers/{ledger}", "ledgers"},
	{"createLedger", http.MethodPost, "/v1/ledgers/{ledger}", "ledgers"},
	{"replaceLedgerMetadata", http.MethodPut, "/v1/ledgers/{ledger}/metadata", "ledgers"},
	{"getLedgerStats", http.MethodGet, "/v1/ledgers/{ledger}/stats", "ledgers"},
	{"listAssets", http.MethodGet, "/v1/ledgers/{ledger}/assets", "ledgers"},
	{"search", http.MethodGet, "/v1/ledgers/{ledger}/search", "ledgers"},
	{"listAccounts", http.MethodGet, "/v1/ledgers/{ledger}/accounts", "accounts"},
	{"getAccount", http.MethodGet, "/v1/ledgers/{ledger}/accounts/{address}", "accounts"},
	{"replaceAccountMetadata", http.MethodPut, "/v1/ledgers/{ledger}/accounts/{address}/metadata", "accounts"},
	{"getAccountHistory", http.MethodGet, "/v1/ledgers/{ledger}/accounts/{address}/history", "accounts"},
	{"aggregateBalances", http.MethodGet, "/v1/ledgers/{ledger}/aggregate/balances", "accounts"},
	{"listSegments", http.MethodGet, "/v1/ledgers/{ledger}/aggregate/segments", "accounts"},
	{"listTransactions", http.MethodGet, "/v1/ledgers/{ledger}/transactions", "transactions"},
	{"createTransaction", http.MethodPost, "/v1/ledgers/{ledger}/transactions", "transactions"},
	{"getTransaction", http.MethodGet, "/v1/ledgers/{ledger}/transactions/{id}", "transactions"},
	{"replaceTransactionMetadata", http.MethodPut, "/v1/ledgers/{ledger}/transactions/{id}/metadata", "transactions"},
	{"revertTransaction", http.MethodPost, "/v1/ledgers/{ledger}/transactions/{id}/revert", "transactions"},
	{"listWriteRequests", http.MethodGet, "/v1/ledgers/{ledger}/requests", "transactions"},
	{"getWriteRequest", http.MethodGet, "/v1/ledgers/{ledger}/requests/{key}", "transactions"},
	{"listHolds", http.MethodGet, "/v1/ledgers/{ledger}/holds", "holds"},
	{"createHold", http.MethodPost, "/v1/ledgers/{ledger}/holds", "holds"},
	{"getHold", http.MethodGet, "/v1/ledgers/{ledger}/holds/{id}", "holds"},
	{"captureHold", http.MethodPost, "/v1/ledgers/{ledger}/holds/{id}/capture", "holds"},
	{"voidHold", http.MethodPost, "/v1/ledgers/{ledger}/holds/{id}/void", "holds"},
	{"listRules", http.MethodGet, "/v1/ledgers/{ledger}/rules", "settings"},
	{"matchRule", http.MethodGet, "/v1/ledgers/{ledger}/rules/match", "settings"},
	{"putRule", http.MethodPut, "/v1/ledgers/{ledger}/rules/{pattern}", "settings"},
	{"deleteRule", http.MethodDelete, "/v1/ledgers/{ledger}/rules/{pattern}", "settings"},
	{"listEventTypes", http.MethodGet, "/v1/ledgers/{ledger}/event-types", "settings"},
	{"createEventType", http.MethodPost, "/v1/ledgers/{ledger}/event-types", "settings"},
	{"listOrganizations", http.MethodGet, "/v1/organizations", "organizations"},
	{"createOrganization", http.MethodPost, "/v1/organizations", "organizations"},
	{"getOrganization", http.MethodGet, "/v1/organizations/{organization}", "organizations"},
	{"updateOrganization", http.MethodPatch, "/v1/organizations/{organization}", "organizations"},
	{"listMembers", http.MethodGet, "/v1/members", "members"},
	{"inviteMember", http.MethodPost, "/v1/members", "members"},
	{"setMemberRole", http.MethodPut, "/v1/members/{subject}/role", "members"},
	{"removeMember", http.MethodDelete, "/v1/members/{subject}", "members"},
	{"setLedgerRole", http.MethodPut, "/v1/ledgers/{ledger}/members/{subject}", "members"},
	{"revokeLedgerRole", http.MethodDelete, "/v1/ledgers/{ledger}/members/{subject}", "members"},
	{"resendInvitation", http.MethodPost, "/v1/invitations/{invitation}/resend", "members"},
	{"deleteInvitation", http.MethodDelete, "/v1/invitations/{invitation}", "members"},
}

func TestOperationsAreRegistered(t *testing.T) {
	t.Parallel()
	api := restapi.NewAPI(http.NewServeMux())
	RegisterAll(api, Handlers{})
	found := map[string]bool{}
	for path, item := range api.OpenAPI().Paths {
		for method, op := range map[string]*huma.Operation{
			http.MethodGet: item.Get, http.MethodPost: item.Post, http.MethodPut: item.Put, http.MethodDelete: item.Delete,
			http.MethodPatch: item.Patch,
		} {
			if op != nil {
				found[method+" "+path+" "+op.OperationID+" "+strings.Join(op.Tags, ",")] = true
			}
		}
	}
	for _, r := range registered {
		key := r.method + " " + r.path + " " + r.id + " " + r.tag
		if !found[key] {
			t.Errorf("operation %s is not registered as %s", r.id, key)
		}
		delete(found, key)
	}
	for extra := range found {
		t.Errorf("unexpected operation %s", extra)
	}
}

// TestHealthAloneWhenRESTIsDisabled checks what bootstrap serves with REST_ENABLED=false.
func TestHealthAloneWhenRESTIsDisabled(t *testing.T) {
	t.Parallel()
	api := restapi.NewAPI(http.NewServeMux())
	RegisterHealth(api, Handlers{}.Health)
	var paths []string
	for path := range api.OpenAPI().Paths {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	if want := []string{"/healthz", "/readyz", "/v1/_info"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}
