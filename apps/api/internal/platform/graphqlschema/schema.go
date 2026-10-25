// Package graphqlschema renders a GraphQL SDL document from Go types the modules register
// (ADR-0020, plan section 8). gqlgen is schema-first; this package closes the gap: modules
// register typed Go structs, cmd/graphqlschema turns them into SDL, gqlgen turns the SDL into
// typed resolver interfaces, and the modules implement them. It imports neither gqlgen nor any
// generated code, so cmd/graphqlschema compiles even when the gqlgen output is stale.
//
// Go note: Go methods cannot have their own type parameters, so the registration calls are
// generic functions that take the schema as their first argument. reflect.TypeFor[T]() turns a
// type parameter into a reflect.Type without needing a value.
package graphqlschema

import (
	"reflect"
	"time"
)

// Schema collects registrations. It has no package-level state: build one with New.
type Schema struct {
	// fields holds the root fields ("Query", "Mutation") and the resolver fields added to object
	// types, by GraphQL type name, in registration order.
	fields  map[string][]field
	order   []string
	scalars map[reflect.Type]scalarDef
	docs    map[reflect.Type]string
}

type field struct {
	name, doc    string
	args, result reflect.Type
}

type scalarDef struct {
	name, doc string
}

// NoArgs marks a field without arguments.
type NoArgs struct{}

// PageInfo is the page information of every connection (plan section 8.6). endCursor is exactly
// the opaque cursor REST returns as next, so a cursor taken from one transport works on the
// other.
type PageInfo struct {
	EndCursor   *string `json:"endCursor" doc:"Pass as after to read the next page. Null on the last page."`
	HasNextPage bool    `json:"hasNextPage"`
}

// PageInfoOf turns the cursor of the next page into page information. The cursor is the one REST
// returns as next, so a cursor from one transport works on the other.
func PageInfoOf(next string) PageInfo {
	if next == "" {
		return PageInfo{}
	}
	return PageInfo{EndCursor: &next, HasNextPage: true}
}

// New returns an empty schema that already maps time.Time to DateTime and int64 to Int64.
func New() *Schema {
	s := &Schema{
		fields:  map[string][]field{},
		scalars: map[reflect.Type]scalarDef{},
		docs:    map[reflect.Type]string{},
	}
	Scalar[time.Time](s, "DateTime", "RFC 3339 date-time.")
	Scalar[int64](s, "Int64", "64-bit integer as a JSON number. Counts only, never money.")
	Scalar[uint32](s, "Int64", "64-bit integer as a JSON number. Counts only, never money.")
	return s
}

// Query registers Query.<name>(args): Result.
func Query[Args, Result any](s *Schema, name, doc string) {
	s.add("Query", name, doc, reflect.TypeFor[Args](), reflect.TypeFor[Result]())
}

// Mutation registers Mutation.<name>(args): Result.
func Mutation[Args, Result any](s *Schema, name, doc string) {
	s.add("Mutation", name, doc, reflect.TypeFor[Args](), reflect.TypeFor[Result]())
}

// Field registers a resolver field on an object type named by its GraphQL name. Naming the parent
// by string lets one module extend a type another module owns (Ledger.accounts) without importing
// it; SDL fails when the parent type does not exist.
func Field[Args, Result any](s *Schema, parent, name, doc string) {
	s.add(parent, name, doc, reflect.TypeFor[Args](), reflect.TypeFor[Result]())
}

// Scalar maps a Go type to a custom scalar.
func Scalar[T any](s *Schema, name, doc string) {
	s.scalars[reflect.TypeFor[T]()] = scalarDef{name: name, doc: doc}
}

// Describe sets the description of an object, input or enum type.
func Describe[T any](s *Schema, doc string) {
	s.docs[reflect.TypeFor[T]()] = doc
}

func (s *Schema) add(parent, name, doc string, args, result reflect.Type) {
	if _, ok := s.fields[parent]; !ok {
		s.order = append(s.order, parent)
	}
	s.fields[parent] = append(s.fields[parent], field{name: name, doc: doc, args: args, result: result})
}
