// Command openapi writes the REST spec, packages/openapi/openapi.yaml, from the Go registrations
// (ADR-0018). It builds the same Huma API as the server and never starts it: no socket, no
// TigerBeetle, no Postgres. `just generate` runs it with CGO_ENABLED=0, so it fails to build if the
// cgo TigerBeetle client ever becomes one of its dependencies.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	v1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

func main() {
	out := flag.String("out", "", "file to write the spec to; standard output when empty")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "openapi:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	spec, err := build()
	if err != nil {
		return err
	}
	if out == "" {
		_, err = os.Stdout.Write(spec)
		return err
	}
	if err := os.WriteFile(out, spec, 0o644); err != nil { //nolint:gosec // the spec is a public file
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

// build registers every operation with nil use cases: registering never calls one.
func build() ([]byte, error) {
	api := restapi.NewAPI(http.NewServeMux())
	v1.Register(api, v1.Deps{})
	return restapi.Spec(api)
}
