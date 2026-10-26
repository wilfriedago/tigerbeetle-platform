// Package restapi builds the Huma API that serves REST and writes the REST spec (ADR-0018). The
// server and cmd/openapi call the same NewAPI, so the spec describes exactly what is served. This
// package opens no connection and registers no operation of its own: the handlers register theirs
// through Register.
package restapi

import (
	"net/http"
	"reflect"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// SpecVersion is info.version of the REST contract. It changes when the contract changes, not
// with every build of the service.
const SpecVersion = "0.2.0"

// hooks guards Huma's package-level settings. Huma reads its error model and its array
// nullability from package variables; they are assigned once, here, and never change afterwards.
// This is the one exception to the "no globals" rule in AGENTS.md (ADR-0018). Go note: sync.Once
// runs the function at most once even when tests build several APIs in parallel, which keeps the
// race detector quiet.
var hooks sync.Once

func configureHuma() {
	hooks.Do(func() {
		// Slices are arrays, never null, as in the committed spec.
		huma.DefaultArrayNullable = false
		huma.NewError = newHumaError
		huma.NewErrorWithContext = newHumaErrorWithContext
	})
}

// integer stands in for Go's int in the spec. Huma gives every int `format: int64`, while the
// committed spec declares plain integers for codes, counts and limits; oasdiff counts a format
// added to a request field as a breaking change. The convention matches the one oapi-codegen
// used: a Go int is a JSON integer, a Go int64 is `format: int64`.
type integer int

// Schema returns a fresh schema each time, because Huma writes field constraints onto it.
func (integer) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: huma.TypeInteger}
}

// NewAPI returns the Huma API on mux, configured as the committed spec describes it.
func NewAPI(mux *http.ServeMux) huma.API {
	configureHuma()
	cfg := huma.DefaultConfig("TigerBeetle Platform API", SpecVersion)
	// No spec route, no docs page and no JSON schema routes: the committed file is the published
	// contract, and the docs page would load JavaScript from a CDN.
	cfg.OpenAPIPath, cfg.DocsPath, cfg.SchemasPath = "", "", ""
	// The default create hook adds a $schema member to every body and a Link header.
	cfg.CreateHooks = nil
	cfg.Info.Description = specDescription
	cfg.Info.License = &huma.License{Name: "Apache-2.0", Identifier: "Apache-2.0"}
	// OpenAPI 3.1 fields Huma has no struct field for go in through Extensions.
	cfg.Info.Extensions = map[string]any{"summary": "A ledger API that makes TigerBeetle easier to work with."}
	cfg.Extensions = map[string]any{"security": []any{}}
	cfg.Servers = []*huma.Server{{URL: "http://localhost:8080", Description: "Local compose stack"}}
	cfg.Tags = specTags()
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		SecurityScheme: {
			Type: "http", Scheme: "bearer", BearerFormat: "JWT",
			Description: "An access token from the platform's identity provider (ADR-0026).",
		},
	}
	cfg.Components.Schemas.RegisterTypeAlias(reflect.TypeFor[int](), reflect.TypeFor[integer]())
	cfg.Transformers = append(cfg.Transformers, problemHeaders)
	cfg.Formats = map[string]huma.Format{"application/json": jsonFormat, "json": jsonFormat}

	api := humago.New(mux, cfg)
	registerComponents(api.OpenAPI())
	return api
}

// SecurityScheme names the bearer token scheme every non-public operation requires.
const SecurityScheme = "bearerAuth"

// specTags lists the tags in their published order. They shape the generated client, one module
// per tag, so they change only on purpose.
func specTags() []*huma.Tag {
	return []*huma.Tag{
		{Name: "operations", Description: "Liveness, readiness, service information and the reconciler."},
		{Name: "ledgers", Description: "Ledgers are namespaces. Money never crosses ledgers."},
		{Name: "accounts", Description: "Accounts are addresses such as `users:1234:wallet`, created by their first posting."},
		{Name: "transactions", Description: "A transaction is one linked chain of TigerBeetle transfers; all postings apply or none."},
		{Name: "holds", Description: "Holds are TigerBeetle two-phase transfers, captured, voided or expired exactly once."},
		{Name: "settings", Description: "Account rules and event types of a ledger."},
	}
}

const specDescription = `The contract for the TigerBeetle Platform API. This file is generated from the Go code by
` + "`cmd/openapi`" + ` (ADR-0018); do not edit it. The TypeScript client is generated from it.

TigerBeetle is the only source of truth for money: every balance, amount and committed time in
a response is read from the cluster. Postgres holds names and context only (ADR-0002).

Amounts are strings holding an integer in the asset's smallest unit (` + "`\"1250\"`" + ` with ` + "`EUR/2`" + ` is
12.50). Balances may be negative and use the signed form. IDs are 32 lowercase hexadecimal
characters, time ordered (ADR-0004).

Writes that move money (transactions, reverts, holds, captures, voids) require an
` + "`Idempotency-Key`" + ` header. A key maps to exactly one outcome: retrying with the same key and
body replays the stored answer, a key reused with a different body answers 422, and a key still
in flight answers 409 with ` + "`Retry-After`" + `. A declined write stays declined under its key; retry
with a new key (ADR-0007).

Errors use RFC 9457 problem details with the ` + "`application/problem+json`" + ` media type. The
` + "`type`" + ` is a stable absolute URI, the same in every deployment, that opens the problem's entry in
the catalogue (` + "`docs/problems.md`" + `): for example
` + "`" + `https://github.com/wilfriedago/tigerbeetle-platform/blob/develop/docs/problems.md#exceeds-credits` + "`" + `.
A problem the HTTP status describes on its own has the type ` + "`about:blank`" + `.

Every operation except the health probes needs an access token, ` + "`Authorization: Bearer <token>`" + `
(ADR-0026), and answers 401 without a valid one. A person names the organization a call acts
for with ` + "`X-Organization`" + `; ledger names are unique within an organization (ADR-0027). A caller
without a role on a ledger gets 404, one whose role lacks the permission 403 (ADR-0028). Request
bodies must be sent as ` + "`application/json`" + `; anything else answers 415 (ADR-0032).
`
