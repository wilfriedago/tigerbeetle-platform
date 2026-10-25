package v1

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"

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
