package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// DefaultGracePeriod is how long a pending record is treated as a write in progress before the
// reconciler settles it (ADR-0007). It is well above the request timeout.
const DefaultGracePeriod = 30 * time.Second

// Stored outcome statuses. The idempotency record keeps "the stored status code and body" so a
// replay answers exactly what the first request answered (ADR-0007).
const (
	statusCreated     = 201
	statusOK          = 200
	statusInvalid     = 400
	statusConflict    = 409
	statusRejected    = 422
	statusInternal    = 500
	statusUnavailable = 503
)

// storedOutcome is the body kept on a resolved record: the created ID, or the problem.
type storedOutcome struct {
	ID      string   `json:"id,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Detail  string   `json:"detail,omitempty"`
	Posting *int     `json:"posting,omitempty"`
	Results []string `json:"results,omitempty"`
}

var kindNames = map[string]error{
	"invalid":     ledger.ErrInvalid,
	"rejected":    ledger.ErrRejected,
	"conflict":    ledger.ErrConflict,
	"not-found":   ledger.ErrNotFound,
	"unavailable": ledger.ErrUnavailable,
}

func kindName(kind error) string {
	for name, k := range kindNames {
		if errors.Is(kind, k) {
			return name
		}
	}
	return "internal"
}

func statusForKind(kind error) int {
	switch {
	case errors.Is(kind, ledger.ErrInvalid):
		return statusInvalid
	case errors.Is(kind, ledger.ErrRejected):
		return statusRejected
	case errors.Is(kind, ledger.ErrConflict):
		return statusConflict
	case errors.Is(kind, ledger.ErrUnavailable):
		return statusUnavailable
	default:
		return statusInternal
	}
}

// problemOutcome encodes a problem for the record.
func problemOutcome(p *ledger.Problem) (status int, body []byte) {
	o := storedOutcome{Kind: kindName(p.Kind), Reason: p.Reason, Detail: p.Detail, Results: p.Results}
	if p.Posting >= 0 {
		posting := p.Posting
		o.Posting = &posting
	}
	body, _ = json.Marshal(o)
	return statusForKind(p.Kind), body
}

// storedProblem rebuilds the problem a declined or failed record answered with.
func storedProblem(rec WriteRecord) error {
	var o storedOutcome
	if err := json.Unmarshal(rec.Response, &o); err != nil || o.Reason == "" {
		return ledger.NewProblem(ledger.ErrRejected, "write-failed", "this Idempotency-Key was used by a write that failed; retry with a new key")
	}
	kind, ok := kindNames[o.Kind]
	if !ok {
		kind = ledger.ErrRejected
	}
	p := &ledger.Problem{Kind: kind, Reason: o.Reason, Detail: o.Detail, Posting: -1, Results: o.Results}
	if o.Posting != nil {
		p.Posting = *o.Posting
	}
	return p
}

// canonicalRequest marshals the normalized request and hashes it. encoding/json writes struct
// fields in declaration order and map keys sorted, so equal requests hash equally.
func canonicalRequest(v any) (body, hash []byte, err error) {
	body, err = json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("encode request: %w", err)
	}
	sum := sha256.Sum256(body)
	return body, sum[:], nil
}

// writeFlow is the protocol shared by every write that moves money (ADR-0007): record first with
// minted IDs, then submit to TigerBeetle, then resolve the record.
type writeFlow struct {
	writes WriteRepository
	tb     LedgerStore
	clock  Clock
	grace  time.Duration
	logger *slog.Logger
}

// begin inserts the pending record. For an existing key it either replays (committed, returning
// the record and ok=false with a nil error), or answers with the problem the key maps to.
func (f *writeFlow) begin(ctx context.Context, rec WriteRecord) (stored WriteRecord, fresh bool, err error) {
	got, created, err := f.writes.BeginWrite(ctx, rec)
	if err != nil {
		return WriteRecord{}, false, fmt.Errorf("begin write: %w", err)
	}
	if created {
		return got, true, nil
	}
	if got.Operation != rec.Operation || !bytes.Equal(got.BodyHash, rec.BodyHash) {
		return got, false, ledger.NewProblem(ledger.ErrRejected, "idempotency-key-reused",
			fmt.Sprintf("Idempotency-Key %q was already used with a different request (%s); a key maps to exactly one write", rec.Key, got.Operation))
	}
	switch got.Status {
	case ledger.RequestPending:
		retry := time.Second
		if age := f.clock.Now().Sub(got.CreatedAt); age > f.grace {
			retry = 5 * time.Second
		}
		return got, false, &ledger.Problem{
			Kind:       ledger.ErrConflict,
			Reason:     "request-in-flight",
			Detail:     "a write with this Idempotency-Key is still in progress; retry after the delay to get its outcome",
			Posting:    -1,
			RetryAfter: retry,
		}
	case ledger.RequestCommitted:
		return got, false, nil
	default:
		return got, false, storedProblem(got)
	}
}

// submit records the attempt and sends the transfers. A TigerBeetle failure leaves the record
// pending: the request may still land, and the reconciler settles it either way.
func (f *writeFlow) submit(ctx context.Context, rec WriteRecord, transfers []TransferSpec) ([]TransferOutcome, error) {
	if err := f.writes.NoteAttempt(ctx, rec.Ledger, rec.Key); err != nil {
		return nil, fmt.Errorf("note attempt: %w", err)
	}
	outcomes, err := f.tb.CreateTransfers(ctx, transfers)
	if err != nil {
		return nil, f.pending(ctx, rec, err)
	}
	if len(outcomes) != len(transfers) {
		return nil, fmt.Errorf("create_transfers returned %d results for %d transfers", len(outcomes), len(transfers))
	}
	return outcomes, nil
}

// pending answers a write whose record stays pending because a store did not answer. The request
// may still land; the reconciler settles the record either way, and a retry with the same key
// returns its outcome. Errors that are not outages pass through unchanged.
func (f *writeFlow) pending(ctx context.Context, rec WriteRecord, err error) error {
	var p *ledger.Problem
	if !errors.Is(err, ledger.ErrUnavailable) || errors.As(err, &p) {
		return err
	}
	f.logger.WarnContext(ctx, "write interrupted; record left pending for the reconciler",
		slog.String("ledger", rec.Ledger), slog.String("idempotency_key", rec.Key), slog.Any("error", err))
	return &ledger.Problem{
		Kind:       ledger.ErrUnavailable,
		Reason:     "write-pending",
		Detail:     "TigerBeetle or Postgres did not answer in time. The write is recorded and will be completed or settled by the reconciler; retry with the same Idempotency-Key to get its outcome",
		Posting:    -1,
		RetryAfter: 5 * time.Second,
	}
}

// decline resolves the record as declined (a valid request TigerBeetle refused) or failed.
func (f *writeFlow) decline(ctx context.Context, rec WriteRecord, status ledger.RequestStatus, p *ledger.Problem) error {
	code, body := problemOutcome(p)
	err := f.writes.ResolveWrite(ctx, rec.Ledger, rec.Key, WriteOutcome{
		Ledger:         rec.Ledger,
		Key:            rec.Key,
		Status:         status,
		ResponseStatus: code,
		Response:       body,
		ProblemType:    p.Reason,
		ResolvedAt:     f.clock.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("resolve write: %w", err)
	}
	return p
}

// committedOutcome is the record update for a successful write.
func (f *writeFlow) committedOutcome(rec WriteRecord, status int, id ledger.ID) WriteOutcome {
	body, _ := json.Marshal(storedOutcome{ID: id.String()})
	return WriteOutcome{
		Ledger:         rec.Ledger,
		Key:            rec.Key,
		Status:         ledger.RequestCommitted,
		ResponseStatus: status,
		Response:       body,
		ResolvedAt:     f.clock.Now().UTC(),
	}
}

// verdict classifies the outcomes of one chain.
type verdict int

const (
	// verdictCommitted: every transfer was created, or the chain had already landed.
	verdictCommitted verdict = iota
	// verdictDeclined: TigerBeetle refused one transfer and the link failed the others.
	verdictDeclined
	// verdictBurned: an ID failed before (id_already_failed), so the original reason is gone.
	verdictBurned
)

type chainVerdict struct {
	verdict verdict
	// failed is the index of the transfer that caused a decline, and result its TigerBeetle result.
	failed int
	result string
	// timestamp is the latest TigerBeetle timestamp of a committed chain, 0 when unknown (exists).
	timestamp uint64
	results   []string
}

// classify reads the outcomes of a linked chain. All created means committed. The pattern
// [exists, linked_event_failed, ...] means a resubmitted chain had already landed: the first
// transfer exists and the link fails the rest without applying them (ADR-0007).
func classify(outcomes []TransferOutcome) chainVerdict {
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
		return chainVerdict{verdict: verdictCommitted, timestamp: ts, results: results, failed: -1}
	}
	if slices.Contains(results, ResultIDAlreadyFailed) {
		return chainVerdict{verdict: verdictBurned, results: results, failed: -1}
	}
	landed := results[0] == ResultExists
	for _, r := range results[1:] {
		if r != ResultExists && r != ResultLinkedEventFailed {
			landed = false
		}
	}
	if landed {
		return chainVerdict{verdict: verdictCommitted, results: results, failed: -1}
	}
	for i, r := range results {
		if r != ResultCreated && r != ResultExists && r != ResultLinkedEventFailed {
			return chainVerdict{verdict: verdictDeclined, failed: i, result: r, results: results}
		}
	}
	// Only linked_event_failed and exists: no transfer explains the failure. Treat it as declined
	// on the first non-created transfer so the client still sees every result.
	for i, r := range results {
		if r != ResultCreated {
			return chainVerdict{verdict: verdictDeclined, failed: i, result: r, results: results}
		}
	}
	return chainVerdict{verdict: verdictDeclined, failed: -1, results: results}
}

// resultReason turns a TigerBeetle result into a problem reason: exceeds_credits becomes
// exceeds-credits.
func resultReason(result string) string {
	return strings.ReplaceAll(result, "_", "-")
}

// latestTimestamp reads the committed time of transfers that already existed.
func latestTimestamp(ctx context.Context, tb LedgerStore, ids []ledger.ID) (uint64, error) {
	found, err := tb.LookupTransfers(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("lookup transfers: %w", err)
	}
	var ts uint64
	for i := range found {
		t := &found[i]
		ts = max(ts, t.Timestamp)
	}
	if ts == 0 {
		return 0, fmt.Errorf("%w: committed transfers not found in TigerBeetle", ledger.ErrUnavailable)
	}
	return ts, nil
}

// timeFromTB converts a TigerBeetle timestamp (nanoseconds since the Unix epoch) to a time.
func timeFromTB(ts uint64) time.Time {
	return time.Unix(0, int64(ts)).UTC()
}

// ensureAccounts resolves every (address, asset) pair to its TigerBeetle account, creating the
// missing ones from the matching rule (ADR-0006). It allocates ledger numbers on first use
// (ADR-0005), inserts candidate rows with ON CONFLICT DO NOTHING, and sends every row, new or
// not, to create_accounts, which heals a crash between the Postgres insert and TigerBeetle.
func ensureAccounts(ctx context.Context, store AccountRepository, rules RuleRepository, tb LedgerStore, ids IDGenerator, clock Clock, ledgerName string, keys []AccountKey) (map[AccountKey]AccountRow, error) {
	keys = uniqueKeys(keys)
	assets := make([]ledger.Asset, 0, len(keys))
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k.Asset] {
			continue
		}
		seen[k.Asset] = true
		a, err := ledger.ParseAsset(k.Asset)
		if err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	assetRows, err := store.EnsureAssets(ctx, ledgerName, assets)
	if err != nil {
		return nil, fmt.Errorf("ensure assets: %w", err)
	}
	existing, err := store.FindAccounts(ctx, ledgerName, keys)
	if err != nil {
		return nil, fmt.Errorf("find accounts: %w", err)
	}
	byKey := make(map[AccountKey]AccountRow, len(keys))
	for i := range existing {
		r := &existing[i]
		byKey[r.Key()] = *r
	}
	var candidates []AccountRow
	var stored []ledger.Rule
	rulesLoaded := false
	now := clock.Now().UTC()
	for _, k := range keys {
		if _, ok := byKey[k]; ok {
			continue
		}
		if !rulesLoaded {
			if stored, err = rules.ListRules(ctx, ledgerName); err != nil {
				return nil, fmt.Errorf("list rules: %w", err)
			}
			rulesLoaded = true
		}
		rule, _ := ledger.MatchRule(stored, k.Address)
		asset := assetRows[k.Asset]
		candidates = append(candidates, AccountRow{
			ID:          ids.NewID(),
			Ledger:      ledgerName,
			Address:     k.Address,
			Asset:       k.Asset,
			TBLedger:    asset.TBLedger,
			Code:        rule.Code,
			Limit:       rule.Limit,
			History:     rule.History,
			RulePattern: string(rule.Pattern),
			CreatedAt:   now,
		})
	}
	if len(candidates) > 0 {
		won, err := store.EnsureAccounts(ctx, ledgerName, candidates)
		if err != nil {
			return nil, fmt.Errorf("ensure accounts: %w", err)
		}
		for i := range won {
			r := &won[i]
			byKey[r.Key()] = *r
		}
	}
	specs := make([]AccountSpec, 0, len(keys))
	for _, k := range keys {
		r, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("account %s %s missing after insert", k.Address, k.Asset)
		}
		specs = append(specs, AccountSpec{ID: r.ID, TBLedger: r.TBLedger, Code: r.Code, Limit: r.Limit, History: r.History})
	}
	if err := tb.CreateAccounts(ctx, specs); err != nil {
		return nil, fmt.Errorf("create accounts: %w", err)
	}
	return byKey, nil
}

func uniqueKeys(keys []AccountKey) []AccountKey {
	seen := make(map[AccountKey]bool, len(keys))
	out := make([]AccountKey, 0, len(keys))
	for _, k := range keys {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}
