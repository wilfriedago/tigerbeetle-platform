package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Problem type URIs. "about:blank" is the RFC 9457 default for problems fully described by their
// HTTP status; domain problems get a stable relative URI under /problems so clients can switch on
// them without parsing titles. A ledger.Problem's reason becomes "/problems/<reason>".
const (
	problemTypeBlank         = "about:blank"
	problemTypeNotFound      = "/problems/not-found"
	problemTypeAlreadyExists = "/problems/already-exists"
	problemTypeInvalid       = "/problems/invalid-request"
	problemTypeUnavailable   = "/problems/dependency-unavailable"
	problemTypeRejected      = "/problems/rejected"
	problemTypeConflict      = "/problems/conflict"
)

// problemTitles names the reasons clients see most. Others get a title derived from the reason.
var problemTitles = map[string]string{
	"invalid-request":         "Invalid request",
	"missing-idempotency-key": "Idempotency-Key required",
	"idempotency-key-reused":  "Idempotency-Key reused",
	"request-in-flight":       "Request in flight",
	"write-pending":           "Write pending",
	"exceeds-credits":         "Declined: exceeds credits",
	"exceeds-debits":          "Declined: exceeds debits",
	"already-reverted":        "Already reverted",
	"hold-already-resolved":   "Hold already resolved",
	"hold-expired":            "Hold expired",
	"capture-exceeds-hold":    "Capture exceeds hold",
	"prefix-too-broad":        "Prefix too broad",
	"history-not-enabled":     "History not enabled",
	"unknown-event-type":      "Unknown event type",
	"built-in-rule":           "Built-in rule",
	"already-exists":          "Already exists",
}

func titleFor(reason string) string {
	if t, ok := problemTitles[reason]; ok {
		return t
	}
	words := strings.ReplaceAll(reason, "-", " ")
	if words == "" {
		return "Problem"
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

// statusFor maps a domain sentinel to an HTTP status.
func statusFor(err error) int {
	switch {
	case errors.Is(err, ledger.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ledger.ErrAlreadyExists), errors.Is(err, ledger.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ledger.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, ledger.ErrRejected):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ledger.ErrUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// problemDetail extracts the client-safe part of a validation error. Validation messages are
// written by the domain ("invalid: address \"x\" must be ..."); wrapping layers add context in
// front, which is dropped here. Errors of other kinds never expose their text.
func problemDetail(err error) string {
	msg := err.Error()
	marker := ledger.ErrInvalid.Error() + ": "
	if i := strings.LastIndex(msg, marker); i >= 0 {
		return msg[i+len(marker):]
	}
	return ""
}

// problemFor maps an error to a problem. Only the domain error decides the status and title; the
// wrapped driver text never reaches the client (Go architecture rule 5).
func problemFor(err error) (gen.Problem, time.Duration) {
	var dp *ledger.Problem
	if errors.As(err, &dp) {
		p := newProblem(statusFor(dp.Kind), "/problems/"+dp.Reason, titleFor(dp.Reason), dp.Detail)
		if dp.Posting >= 0 {
			posting := dp.Posting
			p.Posting = &posting
		}
		if len(dp.Results) > 0 {
			results := dp.Results
			p.Results = &results
		}
		return p, dp.RetryAfter
	}
	switch {
	case errors.Is(err, ledger.ErrNotFound):
		return newProblem(http.StatusNotFound, problemTypeNotFound, "Not found", ""), 0
	case errors.Is(err, ledger.ErrAlreadyExists):
		return newProblem(http.StatusConflict, problemTypeAlreadyExists, "Already exists", ""), 0
	case errors.Is(err, ledger.ErrInvalid):
		return newProblem(http.StatusBadRequest, problemTypeInvalid, "Invalid request", problemDetail(err)), 0
	case errors.Is(err, ledger.ErrConflict):
		return newProblem(http.StatusConflict, problemTypeConflict, "Conflict", ""), 0
	case errors.Is(err, ledger.ErrRejected):
		return newProblem(http.StatusUnprocessableEntity, problemTypeRejected, "Rejected", ""), 0
	case errors.Is(err, ledger.ErrUnavailable):
		return newProblem(http.StatusServiceUnavailable, problemTypeUnavailable, "Dependency unavailable", ""), 0
	default:
		return newProblem(http.StatusInternalServerError, problemTypeBlank, "Internal server error", ""), 0
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
	p, retryAfter := problemFor(err)
	logger := loggerFrom(r.Context())
	if p.Status >= http.StatusInternalServerError {
		logger.ErrorContext(r.Context(), "request failed", slog.Int("status", p.Status), slog.Any("error", err))
	} else {
		logger.InfoContext(r.Context(), "request rejected", slog.Int("status", p.Status), slog.Any("error", err))
	}
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int((retryAfter+time.Second-1)/time.Second)))
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
