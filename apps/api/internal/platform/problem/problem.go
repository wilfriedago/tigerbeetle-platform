// Package problem maps errors to what a client may see, whatever the transport (ADR-0021). Only the
// domain error decides the kind, reason, title and detail; the text of a wrapped driver error never
// reaches a client. REST renders a Problem as RFC 9457 problem+json; other transports render the
// same fields in their own way.
package problem

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Kind is the class of a problem, named the same on every transport.
type Kind string

// The kinds, one per domain sentinel, plus Internal for everything else.
const (
	KindInvalid       Kind = "INVALID"
	KindNotFound      Kind = "NOT_FOUND"
	KindAlreadyExists Kind = "ALREADY_EXISTS"
	KindConflict      Kind = "CONFLICT"
	KindRejected      Kind = "REJECTED"
	KindUnavailable   Kind = "UNAVAILABLE"
	KindInternal      Kind = "INTERNAL"
)

// TypeBlank is the RFC 9457 type of a problem fully described by its HTTP status.
const TypeBlank = "about:blank"

// typePrefix turns a reason into a problem type: "exceeds-credits" becomes
// "/problems/exceeds-credits", a stable relative URI clients can switch on.
const typePrefix = "/problems/"

// Problem is what a client may learn about a failed call.
type Problem struct {
	Kind Kind
	// Reason is the stable kebab-case reason, such as "exceeds-credits", or "" for a problem whose
	// type is about:blank.
	Reason string
	Title  string
	// Status is the REST status; other transports report it as an informative field.
	Status int
	// Detail is a client-safe explanation of this occurrence, or "".
	Detail string
	// Posting is the zero-based index of the posting that caused the problem, or -1.
	Posting int
	// Results holds the TigerBeetle result of every posting of a declined chain.
	Results []string
	// RetryAfter suggests when to retry, for a write still in flight; zero otherwise.
	RetryAfter time.Duration
}

// Type returns the problem type URI: "/problems/<reason>", or about:blank without a reason.
func (p Problem) Type() string {
	if p.Reason == "" {
		return TypeBlank
	}
	return typePrefix + p.Reason
}

// TypeOf returns the problem type URI of a reason, "" for an empty reason. Stored write records
// keep the reason only.
func TypeOf(reason string) string {
	if reason == "" {
		return ""
	}
	return typePrefix + reason
}

// titles names the reasons clients see most. Others get a title derived from the reason.
var titles = map[string]string{
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

// TitleFor returns the title of a reason: the one in the table, or the reason with its dashes
// turned into spaces and its first letter capitalised ("not-found" becomes "Not found").
func TitleFor(reason string) string {
	if t, ok := titles[reason]; ok {
		return t
	}
	words := strings.ReplaceAll(reason, "-", " ")
	if words == "" {
		return "Problem"
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

// sentinel describes the problem of a bare domain sentinel: its kind, status and generic reason.
type sentinel struct {
	err    error
	kind   Kind
	status int
	reason string
}

// sentinels is checked in order with errors.Is; the first match wins.
var sentinels = []sentinel{
	{ledger.ErrNotFound, KindNotFound, http.StatusNotFound, "not-found"},
	{ledger.ErrAlreadyExists, KindAlreadyExists, http.StatusConflict, "already-exists"},
	{ledger.ErrInvalid, KindInvalid, http.StatusBadRequest, "invalid-request"},
	{ledger.ErrConflict, KindConflict, http.StatusConflict, "conflict"},
	{ledger.ErrRejected, KindRejected, http.StatusUnprocessableEntity, "rejected"},
	{ledger.ErrUnavailable, KindUnavailable, http.StatusServiceUnavailable, "dependency-unavailable"},
}

// From maps err to a problem. A *ledger.Problem keeps its reason, detail, posting, results and
// retry delay. A bare sentinel gets its generic reason, and only ErrInvalid keeps a detail: the part
// of the message the domain wrote. Anything else is an internal error with no detail.
func From(err error) Problem {
	var dp *ledger.Problem
	if errors.As(err, &dp) {
		p := ofSentinel(dp.Kind)
		p.Reason = dp.Reason
		p.Title = TitleFor(dp.Reason)
		p.Detail = dp.Detail
		p.Posting = dp.Posting
		p.Results = dp.Results
		p.RetryAfter = dp.RetryAfter
		return p
	}
	p := ofSentinel(err)
	if p.Kind == KindInvalid {
		p.Detail = invalidDetail(err)
	}
	return p
}

// ofSentinel returns the generic problem of the first sentinel err matches.
func ofSentinel(err error) Problem {
	for _, s := range sentinels {
		if errors.Is(err, s.err) {
			return Problem{Kind: s.kind, Reason: s.reason, Title: TitleFor(s.reason), Status: s.status, Posting: -1}
		}
	}
	return Blank(http.StatusInternalServerError, "Internal server error")
}

// Blank returns an about:blank problem, for answers a transport makes itself (unknown route, wrong
// method, panic).
func Blank(status int, title string) Problem {
	kind := KindInternal
	switch status {
	case http.StatusNotFound:
		kind = KindNotFound
	case http.StatusMethodNotAllowed, http.StatusBadRequest:
		kind = KindInvalid
	}
	return Problem{Kind: kind, Title: title, Status: status, Posting: -1}
}

// invalidDetail extracts the client-safe part of a validation error. The domain writes validation
// messages as "invalid: address \"x\" must be ..."; wrapping layers add context in front, which is
// dropped here.
func invalidDetail(err error) string {
	msg := err.Error()
	marker := ledger.ErrInvalid.Error() + ": "
	if i := strings.LastIndex(msg, marker); i >= 0 {
		return msg[i+len(marker):]
	}
	return ""
}
