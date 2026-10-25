package domain

import (
	"slices"
	"strings"
)

// TigerBeetle results the use cases reason about. The adapter reports every result as the
// snake_case name TigerBeetle documents ("exceeds_credits"), so codes not listed here still reach
// clients with their documented name.
const (
	ResultCreated                      = "created"
	ResultExists                       = "exists"
	ResultLinkedEventFailed            = "linked_event_failed"
	ResultIDAlreadyFailed              = "id_already_failed"
	ResultExceedsCredits               = "exceeds_credits"
	ResultExceedsDebits                = "exceeds_debits"
	ResultPendingTransferAlreadyPosted = "pending_transfer_already_posted"
	ResultPendingTransferAlreadyVoided = "pending_transfer_already_voided"
	ResultPendingTransferExpired       = "pending_transfer_expired"
	ResultPendingTransferNotFound      = "pending_transfer_not_found"
	ResultExceedsPendingTransferAmount = "exceeds_pending_transfer_amount"
)

// Verdict classifies the outcomes of one chain.
type Verdict int

const (
	// VerdictCommitted means every transfer was created, or the chain had already landed.
	VerdictCommitted Verdict = iota
	// VerdictDeclined means TigerBeetle refused one transfer and the link failed the others.
	VerdictDeclined
	// VerdictBurned means an ID failed before (id_already_failed), so the original reason is gone.
	VerdictBurned
)

// ChainVerdict is the classification of a chain's outcomes.
type ChainVerdict struct {
	Verdict Verdict
	// Failed is the index of the transfer that caused a decline, and Result its TigerBeetle result.
	Failed int
	Result string
	// Timestamp is the latest TigerBeetle timestamp of a committed chain, 0 when unknown (exists).
	Timestamp uint64
	Results   []string
}

// Classify reads the outcomes of a linked chain. All created means committed. The pattern
// [exists, linked_event_failed, ...] means a resubmitted chain had already landed: the first
// transfer exists and the link fails the rest without applying them (ADR-0007).
func Classify(outcomes []TransferOutcome) ChainVerdict {
	results := make([]string, len(outcomes))
	var ts uint64
	allCreated := true
	for i, o := range outcomes {
		results[i] = o.Result
		ts = max(ts, o.Timestamp)
		if o.Result != ResultCreated {
			allCreated = false
		}
	}
	if allCreated {
		return ChainVerdict{Verdict: VerdictCommitted, Timestamp: ts, Results: results, Failed: -1}
	}
	if slices.Contains(results, ResultIDAlreadyFailed) {
		return ChainVerdict{Verdict: VerdictBurned, Results: results, Failed: -1}
	}
	landed := results[0] == ResultExists
	for _, r := range results[1:] {
		if r != ResultExists && r != ResultLinkedEventFailed {
			landed = false
		}
	}
	if landed {
		return ChainVerdict{Verdict: VerdictCommitted, Results: results, Failed: -1}
	}
	for i, r := range results {
		if r != ResultCreated && r != ResultExists && r != ResultLinkedEventFailed {
			return ChainVerdict{Verdict: VerdictDeclined, Failed: i, Result: r, Results: results}
		}
	}
	// Only linked_event_failed and exists: no transfer explains the failure. Treat it as declined
	// on the first non-created transfer so the client still sees every result.
	for i, r := range results {
		if r != ResultCreated {
			return ChainVerdict{Verdict: VerdictDeclined, Failed: i, Result: r, Results: results}
		}
	}
	return ChainVerdict{Verdict: VerdictDeclined, Failed: -1, Results: results}
}

// ResultReason turns a TigerBeetle result into a problem reason: exceeds_credits becomes
// exceeds-credits.
func ResultReason(result string) string {
	return strings.ReplaceAll(result, "_", "-")
}
