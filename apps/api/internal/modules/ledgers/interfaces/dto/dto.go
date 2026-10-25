// Package dto holds the wire types of the ledgers module that every transport shares: REST
// answers them, the broker RPCs reply with them, and GraphQL binds to them where its shapes match
// (plan section 8.8). The struct tags carry the descriptions and constraints of both specs.
package dto

import (
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Ledger is a ledger with its metadata, assets and account count.
type Ledger struct {
	Name      scalar.LedgerName `json:"name"`
	Metadata  scalar.Metadata   `json:"metadata"`
	CreatedAt time.Time         `json:"createdAt"`
	Assets    []scalar.Asset    `json:"assets" doc:"The assets used so far, in canonical form."`
	Accounts  int64             `json:"accounts" doc:"The number of addresses in the ledger."`
}

// LedgerFrom renders a ledger.
func LedgerFrom(v application.LedgerView) Ledger {
	return Ledger{
		Name:      scalar.LedgerName(v.Name),
		Metadata:  scalar.MetadataOf(v.Metadata),
		CreatedAt: v.CreatedAt,
		Assets:    scalar.AssetsOf(v.Assets),
		Accounts:  v.Accounts,
	}
}

// Rule decides the flags of the accounts created for the addresses it matches (ADR-0006).
type Rule struct {
	Pattern   string              `json:"pattern"`
	Limit     scalar.BalanceLimit `json:"limit"`
	History   bool                `json:"history"`
	Code      int                 `json:"code" minimum:"1" maximum:"65535"`
	BuiltIn   bool                `json:"builtIn" doc:"A default that applies without being stored."`
	Segments  int                 `json:"segments" doc:"Specificity; the most specific matching rule wins."`
	CreatedAt *time.Time          `json:"createdAt,omitempty"`
	UpdatedAt *time.Time          `json:"updatedAt,omitempty"`
}

// TransformSchema adds the examples of pattern; the example tag holds only one.
func (Rule) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Properties["pattern"].Examples = []any{"users:", "world", "*"}
	return s
}

// RuleFrom renders a rule. Built-in rules have no creation time.
func RuleFrom(r ledger.Rule) Rule {
	out := Rule{
		Pattern:  string(r.Pattern),
		Limit:    scalar.BalanceLimit(r.Limit),
		History:  r.History,
		Code:     int(r.Code),
		BuiltIn:  r.BuiltIn,
		Segments: r.Pattern.Segments(),
	}
	if !r.CreatedAt.IsZero() {
		created, updated := r.CreatedAt, r.UpdatedAt
		out.CreatedAt, out.UpdatedAt = &created, &updated
	}
	return out
}

// RuleCandidate is a rule considered for an address.
type RuleCandidate struct {
	Rule    Rule `json:"rule"`
	Matches bool `json:"matches"`
}

// RuleMatch is the rule that would create the account for an address, and every rule considered.
type RuleMatch struct {
	Address    scalar.Address  `json:"address"`
	Rule       Rule            `json:"rule"`
	Candidates []RuleCandidate `json:"candidates"`
	Existing   []scalar.Asset  `json:"existing" doc:"Assets this address already has accounts in; those keep their frozen flags."`
}

// RuleMatchFrom renders the answer of matchRule.
func RuleMatchFrom(m application.RuleMatch) RuleMatch {
	out := RuleMatch{
		Address: scalar.Address(m.Address), Rule: RuleFrom(m.Rule),
		Candidates: make([]RuleCandidate, len(m.Candidates)), Existing: scalar.AssetsOf(m.Existing),
	}
	for i, c := range m.Candidates {
		out.Candidates[i] = RuleCandidate{Rule: RuleFrom(c.Rule), Matches: c.Matches}
	}
	return out
}

// EventType is an event type of a ledger, carried in the code of every transfer of its writes.
type EventType struct {
	Name         string     `json:"name" example:"card.authorized"`
	Code         int        `json:"code"`
	BuiltIn      bool       `json:"builtIn"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
	Transactions int64      `json:"transactions" doc:"Transactions and holds recorded with this event type."`
}

// EventTypeFrom renders an event type with its usage. The built-in default has no creation time.
func EventTypeFrom(v application.EventTypeView) EventType {
	out := EventType{Name: v.Name, Code: int(v.Code), BuiltIn: v.BuiltIn, Transactions: v.Uses}
	if !v.CreatedAt.IsZero() {
		created := v.CreatedAt
		out.CreatedAt = &created
	}
	return out
}
