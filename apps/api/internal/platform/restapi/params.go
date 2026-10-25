package restapi

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Body wraps an answer for Huma, which takes the response body from a field named Body.
type Body[T any] struct {
	Body T
}

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

// CursorParam documents the cursor parameter of Page. Its description holds backticks, which a Go
// struct tag cannot; Huma keeps a parameter an operation declares itself and still decodes the
// struct field.
func CursorParam() *huma.Param {
	maxLength := 512
	return &huma.Param{
		Name: "cursor", In: "query",
		Description: "The opaque `next` cursor of the previous page.",
		Schema:      &huma.Schema{Type: huma.TypeString, MaxLength: &maxLength},
	}
}

// Inline returns the schema of T in place, without registering it as a component. Types whose
// Schema method calls it appear inline, as anonymous objects. T is a copy of the type without its
// methods, so Huma does not call the same Schema method again.
func Inline[T any](r huma.Registry) *huma.Schema {
	return huma.SchemaFromType(r, reflect.TypeFor[T]())
}

// componentPrefix is where the spec keeps its named schemas.
const componentPrefix = "#/components/schemas/"

// Component registers s under name the first time it is asked for and returns a $ref to it, for
// types that describe themselves, such as an allOf.
func Component(r huma.Registry, name string, s func() *huma.Schema) *huma.Schema {
	if _, ok := r.Map()[name]; !ok {
		r.Map()[name] = s()
	}
	return &huma.Schema{Ref: componentPrefix + name}
}

// Optional returns nil for "", so an absent value is left out of the JSON.
func Optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
