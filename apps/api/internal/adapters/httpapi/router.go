// Package httpapi is the HTTP adapter. oapi-codegen generates the routing, models and strict
// handler plumbing in gen/ from packages/openapi/openapi.yaml; this package implements the
// generated StrictServerInterface by calling app use cases and maps errors to RFC 9457 problems.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
)

// Deps is everything the handler needs. main.go wires it.
type Deps struct {
	Logger       *slog.Logger
	Status       *app.Status
	Ledgers      *app.Ledgers
	Settings     *app.Settings
	Accounts     *app.Accounts
	Transactions *app.Transactions
	Holds        *app.Holds
	Overview     *app.Overview
	Reconciler   *app.Reconciler
	Service      string
	Version      string
	Runtime      Runtime
}

// NewHandler builds the HTTP handler: generated routes on a ServeMux, wrapped so that every error
// path (binding errors, handler errors, unknown routes, panics) answers with problem+json.
func NewHandler(deps Deps) http.Handler {
	srv := &server{
		status:       deps.Status,
		ledgers:      deps.Ledgers,
		settings:     deps.Settings,
		accounts:     deps.Accounts,
		transactions: deps.Transactions,
		holds:        deps.Holds,
		overview:     deps.Overview,
		reconciler:   deps.Reconciler,
		service:      deps.Service,
		version:      deps.Version,
		runtime:      deps.Runtime,
	}

	strict := gen.NewStrictHandlerWithOptions(srv, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, r, newProblem(http.StatusBadRequest, problemTypeInvalid, "Invalid request", err.Error()))
		},
		ResponseErrorHandlerFunc: writeErrorProblem,
	})

	mux := http.NewServeMux()
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: bindingProblem,
	})

	h := unknownRoutes(mux)
	h = recoverPanic(h)
	h = accessLog(h)
	h = requestID(deps.Logger)(h)
	return h
}

// bindingProblem answers parameter binding errors. A missing Idempotency-Key gets its own problem
// type, because clients must learn to send one with every write that moves money (ADR-0007).
func bindingProblem(w http.ResponseWriter, r *http.Request, err error) {
	var missing *gen.RequiredHeaderError
	if errors.As(err, &missing) && missing.ParamName == "Idempotency-Key" {
		writeProblem(w, r, newProblem(http.StatusBadRequest, "/problems/missing-idempotency-key", titleFor("missing-idempotency-key"),
			"every write that moves money requires an Idempotency-Key header; reuse it to retry safely"))
		return
	}
	writeProblem(w, r, newProblem(http.StatusBadRequest, problemTypeInvalid, "Invalid request", err.Error()))
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
