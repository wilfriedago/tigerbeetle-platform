package scalar

import (
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func TestComponentsAreNamedOnce(t *testing.T) {
	t.Parallel()
	types := map[string]reflect.Type{
		"LedgerName":           reflect.TypeFor[LedgerName](),
		"Address":              reflect.TypeFor[Address](),
		"AddressPrefix":        reflect.TypeFor[AddressPrefix](),
		"Asset":                reflect.TypeFor[Asset](),
		"Amount":               reflect.TypeFor[Amount](),
		"SignedAmount":         reflect.TypeFor[SignedAmount](),
		"Id":                   reflect.TypeFor[ID](),
		"TigerBeetleTimestamp": reflect.TypeFor[Timestamp](),
		"Metadata":             reflect.TypeFor[Metadata](),
		"BalanceLimit":         reflect.TypeFor[BalanceLimit](),
		"HoldStatus":           reflect.TypeFor[HoldStatus](),
		"Operation":            reflect.TypeFor[Operation](),
		"RequestStatus":        reflect.TypeFor[RequestStatus](),
		"DependencyStatus":     reflect.TypeFor[DependencyStatus](),
	}
	r := huma.NewMapRegistry(componentPrefix, huma.DefaultSchemaNamer)
	for name, typ := range types {
		first := r.Schema(typ, true, "")
		second := r.Schema(typ, true, "")
		if first.Ref != componentPrefix+name || second.Ref != first.Ref {
			t.Errorf("%s: refs %q and %q, want %q", typ, first.Ref, second.Ref, componentPrefix+name)
		}
		if first == second {
			t.Errorf("%s: the same $ref value was returned twice; Huma writes field descriptions onto it", typ)
		}
	}
	if len(r.Map()) != len(types) {
		t.Fatalf("registry holds %d components, want %d", len(r.Map()), len(types))
	}
	if s := r.Map()["HoldStatus"]; len(s.Enum) != 4 || s.Type != huma.TypeString {
		t.Errorf("HoldStatus = %+v", s)
	}
	if s := r.Map()["Metadata"]; s.Extensions["propertyNames"] == nil || *s.MaxProperties != 64 {
		t.Errorf("Metadata = %+v", s)
	}
	if s := r.Map()["Amount"]; s.Pattern != "^(0|[1-9][0-9]{0,38})$" || len(s.Examples) != 1 {
		t.Errorf("Amount = %+v", s)
	}
}
