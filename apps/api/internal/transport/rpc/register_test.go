package rpc

import (
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/rest"
)

// TestEveryRESTOperationHasAMethod: the methods are v1.<operationId>, one per REST operation
// (plan section 11.2), read from the REST registrations so neither list can drift. The access
// module's operations are the exception: broker RPC serves the platform's own services, and
// organizations, members and credentials are managed by people through REST and GraphQL (auth
// plan, sections 5.6 and 7.1).
func TestEveryRESTOperationHasAMethod(t *testing.T) {
	t.Parallel()
	api := restapi.NewAPI(http.NewServeMux())
	rest.RegisterAll(api, rest.Handlers{})
	var want []string
	for _, item := range api.OpenAPI().Paths {
		for _, op := range []*huma.Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch} {
			if op != nil && !slices.ContainsFunc(op.Tags, accessTag) {
				want = append(want, "v1."+op.OperationID)
			}
		}
	}
	slices.Sort(want)
	got := Router(Modules{}).Methods()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("RPC methods (-REST operations +RPC methods):\n%s", diff)
	}
	if len(got) != 35 {
		t.Errorf("%d methods, want 35", len(got))
	}
}

// accessTag reports whether an operation belongs to the access module, which broker RPC skips.
func accessTag(tag string) bool {
	return slices.Contains([]string{"organizations", "members", "api-keys", "machine-clients"}, tag)
}
