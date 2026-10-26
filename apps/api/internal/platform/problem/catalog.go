package problem

import (
	"net/http"
	"slices"
)

// Definition documents one problem type, as RFC 9457 section 4 asks: the type URI (derived from
// the reason), a short title, the HTTP status it is used with, and when it occurs. The status
// must be the one of the kind the code raises the reason with; the tests of each module pin the
// statuses of its problems.
type Definition struct {
	Reason      string
	Title       string
	Status      int
	Description string
}

// Type returns the problem type URI of the definition.
func (d Definition) Type() string {
	return TypeOf(d.Reason)
}

// catalog lists the problem types this package and the use cases name, in the order
// docs/problems.md presents them. The TigerBeetle results a declined chain reports under their
// own name are defined by the bookkeeping module.
var catalog = []Definition{
	{
		"invalid-request", "Invalid request", http.StatusBadRequest,
		"The request breaks a rule of the API: a malformed parameter or body, or a value the domain refuses. `detail` says " +
			"which; for a parameter or body the service could not decode, `errors` lists each one with its `pointer` (body) " +
			"or `parameter` (path, query or header).",
	},
	{
		"missing-idempotency-key", "Idempotency-Key required", http.StatusBadRequest,
		"A write that moves money (transaction, revert, hold, capture, void) was sent without an `Idempotency-Key` header.",
	},
	{
		"idempotency-key-reused", "Idempotency-Key reused", http.StatusUnprocessableEntity,
		"The `Idempotency-Key` was already used with a different request. A key maps to exactly one write; use a new key " +
			"for a new write.",
	},
	{
		"request-in-flight", "Request in flight", http.StatusConflict,
		"A write with the same `Idempotency-Key` is still running. Retry after the `Retry-After` delay with the same key " +
			"and body to get its outcome.",
	},
	{
		"write-pending", "Write pending", http.StatusServiceUnavailable,
		"The write reached TigerBeetle but its outcome is not known yet. Retry after the `Retry-After` delay with the same " +
			"key and body; the reconciler settles it in the meantime.",
	},
	{
		"write-failed", "Write failed", http.StatusUnprocessableEntity,
		"The `Idempotency-Key` belongs to a write that failed before TigerBeetle saw it. Retry with a new key.",
	},
	{
		"transaction-declined", "Transaction declined", http.StatusUnprocessableEntity,
		"TigerBeetle declined this transaction on an earlier attempt and no longer reports why. Nothing was applied; retry " +
			"with a new `Idempotency-Key`.",
	},
	{
		"hold-declined", "Hold declined", http.StatusUnprocessableEntity,
		"TigerBeetle declined this hold on an earlier attempt and no longer reports why. Nothing was reserved; retry with a " +
			"new `Idempotency-Key`.",
	},
	{
		"hold-resolution-declined", "Hold resolution declined", http.StatusUnprocessableEntity,
		"TigerBeetle declined this capture or void on an earlier attempt and no longer reports why. Retry with a new " +
			"`Idempotency-Key`.",
	},
	{
		"exceeds-credits", "Declined: exceeds credits", http.StatusUnprocessableEntity,
		"A posting would take the source account's debits above its credits, which its account rule forbids. Nothing was " +
			"applied: `posting` is the index of the posting that failed and `results` holds the TigerBeetle result of every " +
			"posting.",
	},
	{
		"exceeds-debits", "Declined: exceeds debits", http.StatusUnprocessableEntity,
		"A posting would take the destination account's credits above its debits, which its account rule forbids. Nothing " +
			"was applied: `posting` and `results` say where.",
	},
	{
		"already-reverted", "Already reverted", http.StatusConflict,
		"The transaction was already reverted. A transaction is reverted at most once.",
	},
	{
		"hold-already-resolved", "Hold already resolved", http.StatusConflict,
		"The hold was already captured or voided. A hold is resolved exactly once.",
	},
	{
		"hold-expired", "Hold expired", http.StatusConflict,
		"The hold expired on the cluster clock and TigerBeetle released it; it can no longer be captured or voided.",
	},
	{
		"capture-exceeds-hold", "Capture exceeds hold", http.StatusBadRequest,
		"The capture asks for more than the amount the hold reserved.",
	},
	{
		"prefix-too-broad", "Prefix too broad", http.StatusUnprocessableEntity,
		"The address prefix covers more accounts than one aggregate may read. Narrow the prefix.",
	},
	{
		"history-not-enabled", "History not enabled", http.StatusUnprocessableEntity,
		"The account was created without the history flag, so TigerBeetle keeps no balance history for it. The flag cannot " +
			"be added to an existing account.",
	},
	{
		"unknown-event-type", "Unknown event type", http.StatusBadRequest,
		"The event type is not registered in the ledger. Create it under the ledger's event types first.",
	},
	{
		"built-in-rule", "Built-in rule", http.StatusBadRequest,
		"The account rule is a built-in default and cannot be deleted; put a rule with the same pattern to replace it.",
	},
	{
		"not-found", "Not found", http.StatusNotFound,
		"The ledger, account, transaction, hold, rule, event type or request does not exist.",
	},
	{
		"already-exists", "Already exists", http.StatusConflict,
		"A resource with the same name or pattern already exists.",
	},
	{
		"conflict", "Conflict", http.StatusConflict,
		"The request conflicts with the current state of the resource.",
	},
	{
		"rejected", "Rejected", http.StatusUnprocessableEntity,
		"The request is valid, but the ledger refuses it.",
	},
	{
		"dependency-unavailable", "Dependency unavailable", http.StatusServiceUnavailable,
		"Postgres or TigerBeetle did not answer in time. Retry later; a write keeps its outcome under its " +
			"`Idempotency-Key`.",
	},
	{
		"not-ready", "Not ready", http.StatusServiceUnavailable,
		"A dependency the service needs is down, so this instance should not receive traffic. `dependencies` reports each " +
			"one.",
	},
	{
		"unknown-method", "Unknown method", http.StatusNotFound,
		"The broker RPC message names a method the service does not have.",
	},
	{
		"deadline-exceeded", "Deadline exceeded", http.StatusServiceUnavailable,
		"The broker RPC deadline passed before the request ran; it was not executed.",
	},
}

// Definitions returns the problem types this package names, in catalogue order.
func Definitions() []Definition {
	return slices.Clone(catalog)
}

// definition returns the catalogued definition of a reason.
func definition(reason string) (Definition, bool) {
	i := slices.IndexFunc(catalog, func(d Definition) bool { return d.Reason == reason })
	if i < 0 {
		return Definition{}, false
	}
	return catalog[i], true
}
