// Package scalar holds the named types that travel on the wire: ledger names, addresses, assets,
// amounts, IDs, timestamps, metadata and the string enums. They are plain strings and maps at run
// time. Each one describes itself once as a named component of the REST spec (ADR-0018), so the
// generated spec keeps the component names clients import, and the GraphQL schema reuses the same
// types for its scalars and enums (ADR-0020).
package scalar

import (
	"github.com/danielgtaylor/huma/v2"
)

// componentPrefix is where the REST spec keeps its named schemas.
const componentPrefix = "#/components/schemas/"

// named registers s under name the first time it is asked for, and returns a $ref to it. Huma
// calls a type's Schema method for every field of that type, so the registration must be
// idempotent; the returned $ref is a fresh value because Huma writes field descriptions onto it.
func named(r huma.Registry, name string, s *huma.Schema) *huma.Schema {
	if _, ok := r.Map()[name]; !ok {
		r.Map()[name] = s
	}
	return &huma.Schema{Ref: componentPrefix + name}
}

// stringSchema describes a string component.
func stringSchema(description, pattern string, maxLength int, examples ...any) *huma.Schema {
	s := &huma.Schema{Type: huma.TypeString, Description: description, Pattern: pattern, Examples: examples}
	if maxLength > 0 {
		s.MaxLength = &maxLength
	}
	return s
}

// LedgerName is a ledger name: the namespace in every API path.
type LedgerName string

// Schema describes LedgerName in the REST spec.
func (LedgerName) Schema(r huma.Registry) *huma.Schema {
	return named(r, "LedgerName", stringSchema(
		"Lowercase letters, digits, dash and underscore, 1 to 63 characters, starting with a letter or digit.",
		"^[a-z0-9][a-z0-9_-]{0,62}$", 0, "eu-payments"))
}

// Address is an account address such as users:1234:wallet.
type Address string

// Schema describes Address in the REST spec.
func (Address) Schema(r huma.Registry) *huma.Schema {
	return named(r, "Address", stringSchema(
		"Colon-separated segments of letters, digits, dash and underscore.",
		"^[a-zA-Z0-9_-]+(:[a-zA-Z0-9_-]+)*$", 255, "users:1234:wallet"))
}

// AddressPrefix is an address prefix on whole segments, with an optional trailing colon.
type AddressPrefix string

// Schema describes AddressPrefix in the REST spec.
func (AddressPrefix) Schema(r huma.Registry) *huma.Schema {
	return named(r, "AddressPrefix", stringSchema(
		"An address prefix on whole segments, with an optional trailing colon.",
		"^[a-zA-Z0-9_-]+(:[a-zA-Z0-9_-]+)*:?$", 256, "users:", "users:1234"))
}

// Asset is an asset in its CODE/SCALE form.
type Asset string

// Schema describes Asset in the REST spec.
func (Asset) Schema(r huma.Registry) *huma.Schema {
	return named(r, "Asset", stringSchema(
		"`CODE/SCALE`. A missing scale means zero: `EUR` is `EUR/0`, not `EUR/2` (ADR-0005).",
		"^[A-Z][A-Z0-9]{1,15}(/[0-9]{1,2})?$", 0, "EUR/2", "XOF/0"))
}

// Amount is an unsigned 128-bit integer in the asset's smallest unit, as a decimal string, so no
// client ever reads money through a float (ADR-0004).
type Amount string

// Schema describes Amount in the REST spec.
func (Amount) Schema(r huma.Registry) *huma.Schema {
	return named(r, "Amount", stringSchema(
		"An unsigned integer in the asset's smallest unit, up to 2^128 - 1.",
		"^(0|[1-9][0-9]{0,38})$", 0, "125000"))
}

// SignedAmount is a signed balance in the asset's smallest unit, as a decimal string.
type SignedAmount string

// Schema describes SignedAmount in the REST spec.
func (SignedAmount) Schema(r huma.Registry) *huma.Schema {
	return named(r, "SignedAmount", stringSchema(
		"A signed integer in the asset's smallest unit, for balances.",
		"^-?(0|[1-9][0-9]{0,39})$", 0, "-481230000"))
}

// ID is a TigerBeetle 128-bit ID as 32 lowercase hexadecimal characters, big-endian, time ordered.
// Its REST component is called Id, as in the spec clients already use.
type ID string

// Schema describes ID in the REST spec.
func (ID) Schema(r huma.Registry) *huma.Schema {
	return named(r, "Id", stringSchema(
		"A 128-bit time-ordered ID as 32 lowercase hexadecimal characters.",
		"^[0-9a-f]{32}$", 0, "0192f3a1b7c94e2a8d5f6c0b1e2d7c2e"))
}

// Timestamp is a TigerBeetle timestamp, nanoseconds since the Unix epoch, as a decimal string.
type Timestamp string

// Schema describes Timestamp in the REST spec, under the name TigerBeetleTimestamp.
func (Timestamp) Schema(r huma.Registry) *huma.Schema {
	return named(r, "TigerBeetleTimestamp", stringSchema(
		"The TigerBeetle timestamp, nanoseconds since the Unix epoch, as a decimal string.",
		"^[0-9]+$", 0))
}

// Metadata is key/value pairs for the client's own systems, stored in Postgres. The domain checks
// the keys, the values and the count; the schema documents the same limits.
type Metadata map[string]string

// Schema describes Metadata in the REST spec. Huma has no field for propertyNames, so the keyword
// goes in through Extensions, which are written next to the other keywords.
func (Metadata) Schema(r huma.Registry) *huma.Schema {
	maxProperties, maxLength := 64, 512
	return named(r, "Metadata", &huma.Schema{
		Type:                 huma.TypeObject,
		Description:          "Key/value pairs for your own systems, stored in Postgres. At most 64 entries.",
		MaxProperties:        &maxProperties,
		AdditionalProperties: &huma.Schema{Type: huma.TypeString, MaxLength: &maxLength},
		Extensions:           map[string]any{"propertyNames": map[string]any{"pattern": "^[A-Za-z0-9_.:-]{1,64}$"}},
	})
}

// enumSchema describes a string enum component.
func enumSchema(description string, values []string) *huma.Schema {
	enum := make([]any, len(values))
	for i, v := range values {
		enum[i] = v
	}
	return &huma.Schema{Type: huma.TypeString, Description: description, Enum: enum}
}

// BalanceLimit is the balance limit flag of the TigerBeetle accounts a rule creates.
type BalanceLimit string

// EnumValues lists the values of BalanceLimit.
func (BalanceLimit) EnumValues() []string {
	return []string{"none", "debits_must_not_exceed_credits", "credits_must_not_exceed_debits"}
}

// Schema describes BalanceLimit in the REST spec.
func (b BalanceLimit) Schema(r huma.Registry) *huma.Schema {
	return named(r, "BalanceLimit", enumSchema("The balance limit flag of TigerBeetle accounts created by a rule.", b.EnumValues()))
}

// HoldStatus is the state of a hold; expired is derived from the cluster clock.
type HoldStatus string

// EnumValues lists the values of HoldStatus.
func (HoldStatus) EnumValues() []string {
	return []string{"open", "captured", "voided", "expired"}
}

// Schema describes HoldStatus in the REST spec.
func (h HoldStatus) Schema(r huma.Registry) *huma.Schema {
	return named(r, "HoldStatus", enumSchema("", h.EnumValues()))
}

// Operation is the kind of write an idempotency record holds.
type Operation string

// EnumValues lists the values of Operation.
func (Operation) EnumValues() []string {
	return []string{"transaction", "revert", "hold", "capture", "void"}
}

// Schema describes Operation in the REST spec.
func (o Operation) Schema(r huma.Registry) *huma.Schema {
	return named(r, "Operation", enumSchema("", o.EnumValues()))
}

// RequestStatus is the state of an idempotency record.
type RequestStatus string

// EnumValues lists the values of RequestStatus.
func (RequestStatus) EnumValues() []string {
	return []string{"pending", "committed", "declined", "failed"}
}

// Schema describes RequestStatus in the REST spec.
func (s RequestStatus) Schema(r huma.Registry) *huma.Schema {
	return named(r, "RequestStatus", enumSchema("", s.EnumValues()))
}

// DependencyStatus says whether a dependency answered its readiness probe in time.
type DependencyStatus string

// The values of DependencyStatus.
const (
	DependencyUp   DependencyStatus = "up"
	DependencyDown DependencyStatus = "down"
)

// EnumValues lists the values of DependencyStatus.
func (DependencyStatus) EnumValues() []string {
	return []string{string(DependencyUp), string(DependencyDown)}
}

// Schema describes DependencyStatus in the REST spec.
func (d DependencyStatus) Schema(r huma.Registry) *huma.Schema {
	return named(r, "DependencyStatus", enumSchema("Whether a dependency answered the readiness probe in time.", d.EnumValues()))
}
