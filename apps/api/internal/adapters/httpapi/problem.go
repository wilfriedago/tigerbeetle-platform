package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Problem type URIs. "about:blank" is the RFC 9457 default for problems fully described by their
// HTTP status; domain problems get a stable relative URI under /problems so clients can switch on
// them without parsing titles.
const (
	problemTypeBlank         = "about:blank"
	problemTypeNotFound      = "/problems/not-found"
	problemTypeAlreadyExists = "/problems/already-exists"
	problemTypeInvalid       = "/problems/invalid-request"
	problemTypeUnavailable   = "/problems/dependency-unavailable"
)

// problemFor maps an error to a problem. Only the domain error decides the status and title; the
// wrapped driver text never reaches the client (Go architecture rule 5).
func problemFor(err error) gen.Problem {
	switch {
	case errors.Is(err, ledger.ErrNotFound):
		return newProblem(http.StatusNotFound, problemTypeNotFound, "Not found", "")
	case errors.Is(err, ledger.ErrAlreadyExists):
		return newProblem(http.StatusConflict, problemTypeAlreadyExists, "Already exists", "")
	case errors.Is(err, ledger.ErrInvalid):
		return newProblem(http.StatusBadRequest, problemTypeInvalid, "Invalid request", "")
	case errors.Is(err, ledger.ErrUnavailable):
		return newProblem(http.StatusServiceUnavailable, problemTypeUnavailable, "Dependency unavailable", "")
	default:
		return newProblem(http.StatusInternalServerError, problemTypeBlank, "Internal server error", "")
	}
}

func newProblem(status int, typ, title, detail string) gen.Problem {
	p := gen.Problem{Type: typ, Title: title, Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}

// writeProblem sends a problem+json response, stamping the request ID and instance.
func writeProblem(w http.ResponseWriter, r *http.Request, p gen.Problem) {
	if id := RequestIDFrom(r.Context()); id != "" {
		p.RequestId = &id
	}
	instance := r.URL.Path
	p.Instance = &instance

	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		loggerFrom(r.Context()).ErrorContext(r.Context(), "write problem response", slog.Any("error", err))
	}
}

// writeErrorProblem logs the underlying error at the right level and answers with its problem.
func writeErrorProblem(w http.ResponseWriter, r *http.Request, err error) {
	p := problemFor(err)
	logger := loggerFrom(r.Context())
	if p.Status >= http.StatusInternalServerError {
		logger.ErrorContext(r.Context(), "request failed", slog.Int("status", p.Status), slog.Any("error", err))
	} else {
		logger.InfoContext(r.Context(), "request rejected", slog.Int("status", p.Status), slog.Any("error", err))
	}
	writeProblem(w, r, p)
}

// dependencyReason turns a probe error into the short, client-safe reason shown in readiness
// responses. The full error goes to the log only.
func dependencyReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "unreachable"
}
