package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// allowAll lets every call through, for the tests of what the use cases do.
type allowAll struct{}

func (allowAll) Require(context.Context, ledger.Permission, string) error { return nil }

// denyAll refuses every call and counts the checks.
type denyAll struct{ checks *int }

var errDenied = errors.New("denied by the test authorizer")

func (d denyAll) Require(context.Context, ledger.Permission, string) error {
	*d.checks++
	return errDenied
}

// TestEveryUseCaseIsChecked calls every method of every use case the module exports, with zero
// arguments, behind an authorizer that refuses everything. Each call must come back with the
// authorizer's refusal: a method that reaches the real use case first would panic on the nil
// repositories of this test, or answer something else (ADR-0028).
func TestEveryUseCaseIsChecked(t *testing.T) {
	checks := 0
	app := reflect.ValueOf(New(Deps{Authorizer: denyAll{checks: &checks}}))
	methods := 0
	for i := range app.NumField() {
		field := app.Field(i)
		if field.Kind() != reflect.Interface {
			continue
		}
		name := app.Type().Field(i).Name
		for j := range field.NumMethod() {
			method := field.Method(j)
			label := fmt.Sprintf("%s.%s", name, field.Type().Method(j).Name)
			t.Run(label, func(t *testing.T) {
				methods++
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("%s ran without a permission check: %v", label, r)
					}
				}()
				args := make([]reflect.Value, method.Type().NumIn())
				args[0] = reflect.ValueOf(context.Background())
				for k := 1; k < len(args); k++ {
					args[k] = reflect.Zero(method.Type().In(k))
				}
				out := method.Call(args)
				err, _ := out[len(out)-1].Interface().(error)
				if !errors.Is(err, errDenied) {
					t.Fatalf("%s answered %v, want the authorizer's refusal", label, err)
				}
			})
		}
	}
	if methods == 0 || checks < methods {
		t.Fatalf("%d methods, %d checks: every method must ask the authorizer", methods, checks)
	}
}
