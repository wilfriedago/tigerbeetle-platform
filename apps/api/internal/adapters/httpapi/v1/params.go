package v1

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// LedgerPath is the ledger named in every path below /v1/ledgers/{ledger}.
type LedgerPath struct {
	Ledger scalar.LedgerName `path:"ledger" doc:"The ledger name."`
}

// Page is the cursor pagination of list operations (ADR-0004). The domain clamps a limit above
// 100; the spec documents the range without enforcing it (ADR-0018).
type Page struct {
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" minimum:"1" maximum:"100" default:"25" doc:"Page size."`
}

// cursorParam documents the cursor parameter of Page. Its description holds backticks, which a Go
// struct tag cannot; Huma keeps a parameter an operation declares itself and still decodes the
// struct field.
func cursorParam() *huma.Param {
	maxLength := 512
	return &huma.Param{
		Name: "cursor", In: "query",
		Description: "The opaque `next` cursor of the previous page.",
		Schema:      &huma.Schema{Type: huma.TypeString, MaxLength: &maxLength},
	}
}

// inline returns the schema of T in place, without registering it as a component. Types whose
// Schema method calls it appear inline, as the anonymous objects of the committed spec do. T is a
// copy of the type without its methods, so Huma does not call the same Schema method again.
func inline[T any](r huma.Registry) *huma.Schema {
	return huma.SchemaFromType(r, reflect.TypeFor[T]())
}

// AccountPath is the account named in the paths below /accounts/{address}.
type AccountPath struct {
	Address scalar.Address `path:"address"`
}

// addressParam documents the address path parameter of AccountPath, whose description holds
// backticks.
func addressParam(api huma.API) *huma.Param {
	return &huma.Param{
		Name: "address", In: "path", Required: true,
		Description: "The account address, such as `users:1234:wallet`.",
		Schema:      scalar.Address("").Schema(api.OpenAPI().Components.Schemas),
	}
}

// componentPrefix is where the spec keeps its named schemas.
const componentPrefix = "#/components/schemas/"

// component registers s under name the first time it is asked for and returns a $ref to it, for
// types that describe themselves, such as the allOf of Account.
func component(r huma.Registry, name string, s func() *huma.Schema) *huma.Schema {
	if _, ok := r.Map()[name]; !ok {
		r.Map()[name] = s()
	}
	return &huma.Schema{Ref: componentPrefix + name}
}

// ResourcePath is the transaction or hold named in a path by its ID.
type ResourcePath struct {
	ID scalar.ID `path:"id" doc:"A 32-hex ID."`
}

// WriteKey is the Idempotency-Key header every write that moves money carries (ADR-0007).
type WriteKey struct {
	IdempotencyKey string `header:"Idempotency-Key"`
}

// idempotencyKeyParam documents the Idempotency-Key header, whose description spans two lines and
// holds backticks.
func idempotencyKeyParam() *huma.Param {
	minLength, maxLength := 1, 255
	return &huma.Param{
		Name: "Idempotency-Key", In: "header", Required: true,
		Description: "Identifies the write. Reusing it with the same body replays the stored outcome; with a\n" +
			"different body answers 422 `idempotency-key-reused`.\n",
		Schema: &huma.Schema{Type: huma.TypeString, MinLength: &minLength, MaxLength: &maxLength},
	}
}

// requireIdempotencyKey answers a write without its Idempotency-Key the way the API always has,
// before anything else in the request is read.
func requireIdempotencyKey(key string) error {
	if key == "" {
		return ledger.NewProblem(ledger.ErrInvalid, "missing-idempotency-key",
			"every write that moves money requires an Idempotency-Key header; reuse it to retry safely")
	}
	return nil
}
