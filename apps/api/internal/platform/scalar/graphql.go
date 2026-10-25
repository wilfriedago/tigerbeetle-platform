package scalar

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// The GraphQL wire form of the scalars (plan section 8.4). MarshalGQL and UnmarshalGQL are plain
// method signatures that gqlgen looks for, so this package does not import gqlgen. Money, IDs and
// TigerBeetle timestamps accept JSON strings only and reject numbers, so no client can lose
// precision through a float; their format is checked by the domain, as on REST. Ledger names,
// addresses and assets are String in the schema and bind to these named types.

func writeQuoted(w io.Writer, s string) {
	_, _ = io.WriteString(w, strconv.Quote(s))
}

func unmarshalString(v any, what string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%w: %s must be a JSON string, got %T", ledger.ErrInvalid, what, v)
	}
	return s, nil
}

// MarshalGQL writes the ledger name.
func (n LedgerName) MarshalGQL(w io.Writer) { writeQuoted(w, string(n)) }

// UnmarshalGQL reads a ledger name.
func (n *LedgerName) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "a ledger name")
	*n = LedgerName(s)
	return err
}

// MarshalGQL writes the address.
func (a Address) MarshalGQL(w io.Writer) { writeQuoted(w, string(a)) }

// UnmarshalGQL reads an address.
func (a *Address) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "an address")
	*a = Address(s)
	return err
}

// MarshalGQL writes the address prefix.
func (p AddressPrefix) MarshalGQL(w io.Writer) { writeQuoted(w, string(p)) }

// UnmarshalGQL reads an address prefix.
func (p *AddressPrefix) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "an address prefix")
	*p = AddressPrefix(s)
	return err
}

// MarshalGQL writes the asset.
func (a Asset) MarshalGQL(w io.Writer) { writeQuoted(w, string(a)) }

// UnmarshalGQL reads an asset.
func (a *Asset) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "an asset")
	*a = Asset(s)
	return err
}

// MarshalGQL writes the amount as a decimal string.
func (a Amount) MarshalGQL(w io.Writer) { writeQuoted(w, string(a)) }

// UnmarshalGQL reads an amount from a JSON string.
func (a *Amount) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "Amount")
	*a = Amount(s)
	return err
}

// MarshalGQL writes the signed amount as a decimal string.
func (a SignedAmount) MarshalGQL(w io.Writer) { writeQuoted(w, string(a)) }

// UnmarshalGQL reads a signed amount from a JSON string.
func (a *SignedAmount) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "SignedAmount")
	*a = SignedAmount(s)
	return err
}

// MarshalGQL writes the ID as 32 hexadecimal characters.
func (id ID) MarshalGQL(w io.Writer) { writeQuoted(w, string(id)) }

// UnmarshalGQL reads an ID from a JSON string.
func (id *ID) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "U128")
	*id = ID(s)
	return err
}

// MarshalGQL writes the TigerBeetle timestamp as a decimal string.
func (t Timestamp) MarshalGQL(w io.Writer) { writeQuoted(w, string(t)) }

// UnmarshalGQL reads a TigerBeetle timestamp from a JSON string.
func (t *Timestamp) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "U64")
	*t = Timestamp(s)
	return err
}

// MarshalGQL writes the metadata as a JSON object of strings.
func (m Metadata) MarshalGQL(w io.Writer) { writeJSON(w, MetadataOf(m)) }

// UnmarshalGQL reads a JSON object whose values are strings.
func (m *Metadata) UnmarshalGQL(v any) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: Metadata must be a JSON object, got %T", ledger.ErrInvalid, v)
	}
	out := make(Metadata, len(obj))
	for k, val := range obj {
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: metadata value %q must be a string", ledger.ErrInvalid, k)
		}
		out[k] = s
	}
	*m = out
	return nil
}

// JSON is a free-form JSON object: the stored request and response of a write request.
type JSON map[string]any

// MarshalGQL writes the object.
func (j JSON) MarshalGQL(w io.Writer) {
	if j == nil {
		j = JSON{}
	}
	writeJSON(w, j)
}

// UnmarshalGQL reads a JSON object.
func (j *JSON) UnmarshalGQL(v any) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: JSON must be an object, got %T", ledger.ErrInvalid, v)
	}
	*j = obj
	return nil
}

func writeJSON(w io.Writer, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte("null")
	}
	_, _ = w.Write(b)
}

// The enums are written and read as their values; gqlgen checks a value against the schema
// before it reaches UnmarshalGQL.

// MarshalGQL writes the enum value.
func (b BalanceLimit) MarshalGQL(w io.Writer) { writeQuoted(w, string(b)) }

// UnmarshalGQL reads an enum value.
func (b *BalanceLimit) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "BalanceLimit")
	*b = BalanceLimit(s)
	return err
}

// MarshalGQL writes the enum value.
func (h HoldStatus) MarshalGQL(w io.Writer) { writeQuoted(w, string(h)) }

// UnmarshalGQL reads an enum value.
func (h *HoldStatus) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "HoldStatus")
	*h = HoldStatus(s)
	return err
}

// MarshalGQL writes the enum value.
func (o Operation) MarshalGQL(w io.Writer) { writeQuoted(w, string(o)) }

// UnmarshalGQL reads an enum value.
func (o *Operation) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "Operation")
	*o = Operation(s)
	return err
}

// MarshalGQL writes the enum value.
func (s RequestStatus) MarshalGQL(w io.Writer) { writeQuoted(w, string(s)) }

// UnmarshalGQL reads an enum value.
func (s *RequestStatus) UnmarshalGQL(v any) error {
	str, err := unmarshalString(v, "RequestStatus")
	*s = RequestStatus(str)
	return err
}

// MarshalGQL writes the enum value.
func (d DependencyStatus) MarshalGQL(w io.Writer) { writeQuoted(w, string(d)) }

// UnmarshalGQL reads an enum value.
func (d *DependencyStatus) UnmarshalGQL(v any) error {
	s, err := unmarshalString(v, "DependencyStatus")
	*d = DependencyStatus(s)
	return err
}
