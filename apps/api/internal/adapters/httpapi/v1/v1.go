// Package v1 is version 1 of the REST interface, written for Huma (ADR-0018). Each file registers
// the operations of one tag with typed input and output structs; the struct tags carry the
// descriptions, examples, formats and constraints that make up packages/openapi/openapi.yaml. The
// handlers decode, call one use case and map the result; they hold no business rule.
package v1

import (
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
)

// Deps holds the use cases the handlers call. cmd/api wires it; cmd/openapi passes the zero value,
// because registering an operation never calls a use case.
type Deps struct {
	Status       *app.Status
	Ledgers      *app.Ledgers
	Settings     *app.Settings
	Accounts     *app.Accounts
	Transactions *app.Transactions
	Holds        *app.Holds
	Overview     *app.Overview
	Reconciler   *app.Reconciler
	Service      Service
}

// Service describes the running binary for GET /v1/_info.
type Service struct {
	Name                     string
	Version                  string
	StartedAt                time.Time
	TigerBeetleClientVersion string
	TigerBeetleClusterID     string
}

// handlers implements the operations. Its methods have the signature restapi.Register expects.
type handlers struct {
	Deps
}

// Register adds every version 1 operation to api, tag by tag, in the order of the spec.
func Register(api huma.API, deps Deps) {
	h := &handlers{Deps: deps}
	registerOperations(api, h)
}
