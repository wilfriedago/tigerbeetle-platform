package v1

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

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
