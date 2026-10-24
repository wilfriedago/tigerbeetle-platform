// Package httpapi is the HTTP adapter. oapi-codegen generates the routing, models and strict
// handler plumbing in gen/ from packages/openapi/openapi.yaml; this package implements the
// generated StrictServerInterface by calling app use cases and maps errors to RFC 9457 problems.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
)

// Deps is everything the handler needs. main.go wires it.
type Deps struct {
	Logger  *slog.Logger
	Status  *app.Status
	Service string
	Version string
}

// NewHandler builds the HTTP handler: generated routes on a ServeMux, wrapped so that every error
// path (binding errors, handler errors, unknown routes, panics) answers with problem+json.
func NewHandler(deps Deps) http.Handler {
	srv := &server{status: deps.Status, service: deps.Service, version: deps.Version}

	strict := gen.NewStrictHandlerWithOptions(srv, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, r, newProblem(http.StatusBadRequest, problemTypeInvalid, "Invalid request", err.Error()))
		},
		ResponseErrorHandlerFunc: writeErrorProblem,
	})

	mux := http.NewServeMux()
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, r, newProblem(http.StatusBadRequest, problemTypeInvalid, "Invalid request", err.Error()))
		},
	})

	h := unknownRoutes(mux)
	h = recoverPanic(h)
	h = accessLog(h)
	h = requestID(deps.Logger)(h)
	return h
}

// unknownRoutes converts the ServeMux's plain-text 404 and 405 answers into problems. The mux
// handler still runs so headers it sets, such as Allow on a 405, are preserved.
func unknownRoutes(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		rec := &headerOnlyRecorder{header: w.Header()}
		mux.ServeHTTP(rec, r)
		switch rec.status {
		case http.StatusNotFound:
			writeProblem(w, r, newProblem(http.StatusNotFound, problemTypeBlank, "Not found", ""))
		case http.StatusMethodNotAllowed:
			writeProblem(w, r, newProblem(http.StatusMethodNotAllowed, problemTypeBlank, "Method not allowed", ""))
		default:
			// Redirects for path cleaning carry only headers, which are already on w.
			w.WriteHeader(rec.status)
		}
	})
}

// headerOnlyRecorder shares the real header map but drops the body, so the mux's text/plain
// error body is replaced while its headers survive.
type headerOnlyRecorder struct {
	header http.Header
	status int
}

func (h *headerOnlyRecorder) Header() http.Header { return h.header }

func (h *headerOnlyRecorder) WriteHeader(code int) {
	if h.status == 0 {
		h.status = code
	}
}

func (h *headerOnlyRecorder) Write(b []byte) (int, error) {
	if h.status == 0 {
		h.status = http.StatusOK
	}
	return len(b), nil
}
