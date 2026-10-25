// Package httpapi serves the REST API: the version 1 operations of package v1, registered on a
// Huma API over the standard ServeMux, behind the standard-library middleware of platform/restapi
// (ADR-0018). The spec in packages/openapi is generated from the same registrations by
// cmd/openapi.
package httpapi

import (
	"log/slog"
	"net/http"

	v1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// NewHandler builds the HTTP handler of the API. Every error path (undecodable input, handler
// errors, unknown routes, panics) answers with problem+json.
func NewHandler(logger *slog.Logger, deps v1.Deps) http.Handler {
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	v1.Register(api, deps)
	return restapi.Handler(mux, logger)
}
