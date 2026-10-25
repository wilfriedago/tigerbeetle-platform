package graphqlschema

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Fixture types, one per reflection rule of plan section 8.3.

type Amount string

type Metadata map[string]string

type HoldStatus string

func (HoldStatus) EnumValues() []string { return []string{"open", "captured"} }

type Base struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type Account struct {
	Base
	Address  string            `json:"address" doc:"Colon-separated segments."`
	Balance  Amount            `json:"balance"`
	Metadata Metadata          `json:"metadata"`
	Status   HoldStatus        `json:"status"`
	Holds    int64             `json:"holds"`
	Code     int               `json:"code"`
	Closed   bool              `json:"closed"`
	Note     *string           `json:"note,omitempty"`
	Tags     []string          `json:"tags"`
	Maybe    *[]string         `json:"maybe"`
	Gaps     []*string         `json:"gaps"`
	Old      string            `json:"old" deprecated:"use address"`
	Hidden   string            `json:"-"`
	internal string            //nolint:unused // checks that unexported fields are skipped
	Extra    map[string]string `json:"-"`
}

type AccountConnection struct {
	Nodes    []Account `json:"nodes"`
	PageInfo PageInfo  `json:"pageInfo"`
}

type AccountsArgs struct {
	First  *int        `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After  *string     `json:"after"`
	Status *HoldStatus `json:"status" default:"open"`
	Prefix *string     `json:"prefix" default:"users:"`
}

type Ledger struct {
	Name string `json:"name"`
}

type LedgerArgs struct {
	Name string `json:"name"`
}

type CreateInput struct {
	Ledger   string    `json:"ledger"`
	Amount   Amount    `json:"amount"`
	Metadata *Metadata `json:"metadata"`
	Limit    *int      `json:"limit" default:"10"`
}

type CreateArgs struct {
	Input CreateInput `json:"input"`
}

func fixtureSchema() *Schema {
	s := New()
	Scalar[Amount](s, "Amount", "Unsigned 128-bit integer as a decimal string.")
	Scalar[Metadata](s, "Metadata", "Key/value pairs: a JSON object of strings.")
	Describe[Ledger](s, "A ledger is a namespace.\nMoney never crosses ledgers.")
	Describe[HoldStatus](s, "The life cycle of a \"hold\".")
	Query[LedgerArgs, *Ledger](s, "ledger", "A ledger by name, or null when no ledger has that name.")
	Mutation[CreateArgs, Account](s, "create", "")
	Field[AccountsArgs, AccountConnection](s, "Ledger", "accounts", "Accounts in address order.")
	Field[NoArgs, []Account](s, "Ledger", "all", "")
	return s
}

func TestSDLGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		schema func() *Schema
	}{
		{name: "fixtures", schema: fixtureSchema},
		{name: "empty", schema: New},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.schema().SDL()
			if err != nil {
				t.Fatalf("SDL: %v", err)
			}
			again, _ := tt.schema().SDL()
			if !bytes.Equal(got, again) {
				t.Fatal("two renders differ")
			}
			golden := filepath.Join("testdata", tt.name+".graphql")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(golden, got, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with UPDATE_GOLDEN=1 to create it)", err)
			}
			if diff := cmp.Diff(string(want), string(got)); diff != "" {
				t.Fatalf("SDL mismatch (-want +got):\n%s", diff)
			}
			if len(got) > 0 {
				if _, err := gqlparser.LoadSchema(&ast.Source{Name: golden, Input: string(got)}); err != nil {
					t.Fatalf("gqlparser rejects the SDL: %v", err)
				}
			}
		})
	}
}

// Fixtures of the error cases.

type Money struct {
	Amount uint64  `json:"amount"`
	Rate   float64 `json:"rate"`
}

type Both struct {
	Name string `json:"name"`
}

type BothArgs struct {
	In Both `json:"in"`
}

type BadConnection struct {
	Items []Ledger `json:"items"`
}

type BadEnum string

func (BadEnum) EnumValues() []string { return []string{"ok", "not-valid"} }

type BadEnumHolder struct {
	Value BadEnum `json:"value"`
}

type Bytes struct {
	Raw []byte `json:"raw"`
}

func TestSDLErrors(t *testing.T) {
	t.Parallel()
	type Ledger struct {
		Other string `json:"other"`
	}
	s := New()
	Query[NoArgs, Money](s, "money", "")
	Query[BothArgs, Both](s, "both", "")
	Query[NoArgs, BadConnection](s, "connection", "")
	Query[NoArgs, BadEnumHolder](s, "enum", "")
	Query[NoArgs, Bytes](s, "bytes", "")
	Query[NoArgs, map[string]int](s, "untyped", "")
	Query[string, Both](s, "scalarArgs", "")
	Query[LedgerArgs, *Ledger](s, "shadow", "")
	Query[LedgerArgs, *Ledger](s, "shadow", "")
	Field[NoArgs, string](s, "Missing", "field", "")
	_, err := s.SDL()
	if err == nil {
		t.Fatal("SDL accepted every broken registration")
	}
	var got []string
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		got = append(got, e.Error())
	}
	want := []string{
		"Money.Amount: Go type uint64 has no GraphQL mapping; money and IDs use a named scalar type, never a number",
		"Money.Rate: Go type float64 has no GraphQL mapping; money and IDs use a named scalar type, never a number",
		"Query.both.In: struct graphqlschema.Both is used both as an input and as an output; give the input its own type",
		"BadEnumHolder.Value: enum BadEnum value \"not-valid\" is not a valid GraphQL name",
		"Bytes.Raw: []byte has no GraphQL mapping; register a scalar",
		"Query.untyped: Go type map[string]int has no GraphQL mapping; money and IDs use a named scalar type, never a number",
		"Query.scalarArgs: arguments must be a struct, got string",
		"Query.shadow: GraphQL name Ledger is used by both graphqlschema.Ledger and graphqlschema.Ledger",
		"Query.shadow: type Query already has a field shadow",
		"field Missing.field: parent type Missing is not an object type of the schema",
		"type BadConnection: a connection has exactly the fields nodes and pageInfo, got [items]",
	}
	for _, w := range want {
		if !containsErr(got, w) {
			t.Errorf("missing error %q in:\n%s", w, strings.Join(got, "\n"))
		}
	}
}

func containsErr(errs []string, want string) bool {
	for _, e := range errs {
		if e == want {
			return true
		}
	}
	return false
}
