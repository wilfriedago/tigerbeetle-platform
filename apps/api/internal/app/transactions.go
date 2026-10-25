package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// PostingView is a posting of a committed transaction with the amount read from TigerBeetle.
type PostingView struct {
	Source      ledger.Address
	Destination ledger.Address
	Asset       string
	Amount      ledger.Amount
	TransferID  ledger.ID
	TBLedger    uint32
}

// TransactionView is a committed transaction.
type TransactionView struct {
	ID             ledger.ID
	Ledger         string
	Postings       []PostingView
	Reference      string
	EventType      ledger.EventType
	BusinessTime   *time.Time
	Metadata       map[string]string
	CommittedAt    time.Time
	Timestamp      uint64
	Reverts        *ledger.ID
	RevertedBy     *ledger.ID
	IdempotencyKey string
	RequestID      string
}

// CreateTransactionInput is a transaction request after HTTP decoding.
type CreateTransactionInput struct {
	Ledger       string
	Key          string
	RequestID    string
	Postings     []ledger.Posting
	Reference    string
	EventType    string
	BusinessTime *time.Time
	Metadata     map[string]string
}

// transactionRequest is the canonical body stored on the idempotency record. The reconciler
// rebuilds the chain from it. Reverts names the reverted transaction for a revert; the unique
// index write_requests_one_revert reads it.
type transactionRequest struct {
	Postings     []postingRequest  `json:"postings,omitempty"`
	Reverts      string            `json:"reverts,omitempty"`
	Reference    string            `json:"reference,omitempty"`
	EventType    string            `json:"eventType,omitempty"`
	BusinessTime *time.Time        `json:"businessTime,omitempty"`
	Metadata     map[string]string `json:"metadata"`
}

type postingRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Asset       string `json:"asset"`
	Amount      string `json:"amount"`
}

// Transactions runs the transaction write path of ADR-0007 and reads committed transactions.
type Transactions struct {
	ledgers  LedgerRepository
	events   EventTypeRepository
	rules    RuleRepository
	accounts AccountRepository
	txs      TransactionRepository
	flow     *writeFlow
	tb       LedgerStore
	ids      IDGenerator
	clock    Clock
	logger   *slog.Logger
}

// TransactionsDeps groups the dependencies of the transactions use case.
type TransactionsDeps struct {
	Store       MetadataStore
	TigerBeetle LedgerStore
	IDs         IDGenerator
	Clock       Clock
	GracePeriod time.Duration
	Logger      *slog.Logger
}

// NewTransactions builds the use case.
func NewTransactions(d TransactionsDeps) *Transactions {
	grace := d.GracePeriod
	if grace <= 0 {
		grace = DefaultGracePeriod
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Transactions{
		ledgers:  d.Store,
		events:   d.Store,
		rules:    d.Store,
		accounts: d.Store,
		txs:      d.Store,
		flow:     &writeFlow{writes: d.Store, tb: d.TigerBeetle, clock: d.Clock, grace: grace, logger: logger},
		tb:       d.TigerBeetle,
		ids:      d.IDs,
		clock:    d.Clock,
		logger:   logger,
	}
}

// Create applies the postings as one linked chain: all or nothing (ADR-0007).
func (u *Transactions) Create(ctx context.Context, in CreateTransactionInput) (TransactionView, error) {
	if err := ledger.ValidateIdempotencyKey(in.Key); err != nil {
		return TransactionView{}, err
	}
	if err := requireLedger(ctx, u.ledgers, in.Ledger); err != nil {
		return TransactionView{}, err
	}
	if err := ledger.ValidatePostings(in.Postings); err != nil {
		return TransactionView{}, err
	}
	if err := ledger.ValidateReference(in.Reference); err != nil {
		return TransactionView{}, err
	}
	if in.Metadata == nil {
		in.Metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(in.Metadata); err != nil {
		return TransactionView{}, err
	}
	event, err := resolveEventType(ctx, u.events, in.Ledger, in.EventType)
	if err != nil {
		return TransactionView{}, err
	}
	req := transactionRequest{Reference: in.Reference, EventType: event.Name, BusinessTime: in.BusinessTime, Metadata: in.Metadata}
	if req.BusinessTime != nil {
		t := req.BusinessTime.UTC().Truncate(time.Millisecond)
		req.BusinessTime = &t
	}
	for _, p := range in.Postings {
		req.Postings = append(req.Postings, postingRequest{Source: string(p.Source), Destination: string(p.Destination), Asset: p.Asset.String(), Amount: p.Amount.String()})
	}
	return u.write(ctx, in.Ledger, in.Key, in.RequestID, ledger.OperationTransaction, req, len(in.Postings))
}

// Revert creates a new transaction with every posting reversed. It is a normal chain and can be
// declined; there is no force option (ADR-0004).
func (u *Transactions) Revert(ctx context.Context, ledgerName, key, requestID, id, reference string, metadata map[string]string) (TransactionView, error) {
	if err := ledger.ValidateIdempotencyKey(key); err != nil {
		return TransactionView{}, err
	}
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return TransactionView{}, err
	}
	txID, err := ledger.ParseID(id)
	if err != nil {
		return TransactionView{}, err
	}
	if err := ledger.ValidateReference(reference); err != nil {
		return TransactionView{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return TransactionView{}, err
	}
	original, err := u.txs.GetTransaction(ctx, ledgerName, txID)
	if err != nil {
		return TransactionView{}, fmt.Errorf("get transaction %s: %w", txID, err)
	}
	req := transactionRequest{Reverts: txID.String(), Reference: reference, EventType: original.EventType, Metadata: metadata}
	// A key that already holds this revert replays it, even though the original now shows as
	// reverted; only a revert under a different key is refused.
	if original.RevertedBy != nil {
		if rec, err := u.flow.writes.GetWrite(ctx, ledgerName, key); err != nil || rec.Operation != ledger.OperationRevert {
			return TransactionView{}, ledger.NewProblem(ledger.ErrConflict, "already-reverted",
				fmt.Sprintf("transaction %s was already reverted by %s; a transaction is reverted at most once", txID, original.RevertedBy))
		}
	}
	return u.write(ctx, ledgerName, key, requestID, ledger.OperationRevert, req, len(original.Legs))
}

// write records the request with minted IDs, then executes it, or answers from the record.
func (u *Transactions) write(ctx context.Context, ledgerName, key, requestID string, op ledger.Operation, req transactionRequest, legs int) (TransactionView, error) {
	body, hash, err := canonicalRequest(req)
	if err != nil {
		return TransactionView{}, err
	}
	rec := WriteRecord{
		Ledger:      ledgerName,
		Key:         key,
		Operation:   op,
		Status:      ledger.RequestPending,
		BodyHash:    hash,
		Request:     body,
		TargetID:    u.ids.NewID(),
		TransferIDs: make([]ledger.ID, legs),
		RequestID:   requestID,
		CreatedAt:   u.clock.Now().UTC(),
	}
	for i := range rec.TransferIDs {
		rec.TransferIDs[i] = u.ids.NewID()
	}
	stored, fresh, err := u.flow.begin(ctx, rec)
	if err != nil {
		if errors.Is(err, ledger.ErrConflict) && op == ledger.OperationRevert && !errors.As(err, new(*ledger.Problem)) {
			return TransactionView{}, ledger.NewProblem(ledger.ErrConflict, "already-reverted",
				fmt.Sprintf("transaction %s is already being reverted under another Idempotency-Key", req.Reverts))
		}
		return TransactionView{}, err
	}
	if !fresh {
		return u.Get(ctx, ledgerName, stored.TargetID.String())
	}
	return u.execute(ctx, stored, req)
}

// execute builds the chain from the record and submits it, then resolves the record. The
// reconciler calls it again with the same record when nothing reached the cluster.
func (u *Transactions) execute(ctx context.Context, rec WriteRecord, req transactionRequest) (TransactionView, error) {
	postings, reverts, err := u.resolvePostings(ctx, rec.Ledger, req)
	if err != nil {
		return TransactionView{}, u.failPrepared(ctx, rec, err)
	}
	if len(postings) != len(rec.TransferIDs) {
		return TransactionView{}, fmt.Errorf("record %s has %d transfer IDs for %d postings", rec.Key, len(rec.TransferIDs), len(postings))
	}
	event, err := resolveEventType(ctx, u.events, rec.Ledger, req.EventType)
	if err != nil {
		return TransactionView{}, u.failPrepared(ctx, rec, err)
	}
	keys := make([]AccountKey, 0, 2*len(postings))
	for _, p := range postings {
		keys = append(keys, AccountKey{Address: p.Source, Asset: p.Asset.String()}, AccountKey{Address: p.Destination, Asset: p.Asset.String()})
	}
	accounts, err := ensureAccounts(ctx, u.accounts, u.rules, u.tb, u.ids, u.clock, rec.Ledger, keys)
	if err != nil {
		return TransactionView{}, u.flow.pending(ctx, rec, err)
	}
	var businessMillis uint64
	if req.BusinessTime != nil {
		businessMillis = uint64(req.BusinessTime.UnixMilli())
	}
	transfers := make([]TransferSpec, len(postings))
	legs := make([]LegRow, len(postings))
	for i, p := range postings {
		src := accounts[AccountKey{Address: p.Source, Asset: p.Asset.String()}]
		dst := accounts[AccountKey{Address: p.Destination, Asset: p.Asset.String()}]
		transfers[i] = TransferSpec{
			ID:              rec.TransferIDs[i],
			DebitAccountID:  src.ID,
			CreditAccountID: dst.ID,
			Amount:          p.Amount,
			TBLedger:        src.TBLedger,
			Code:            event.Code,
			Kind:            TransferPosting,
			// Every transfer but the last carries linked; a one-transfer chain sets no flag, which
			// would otherwise fail with linked_event_chain_open (ADR-0007).
			Linked:      i < len(postings)-1,
			UserData128: rec.TargetID,
			UserData64:  businessMillis,
		}
		legs[i] = LegRow{TransferID: rec.TransferIDs[i], Source: src, Destination: dst}
	}
	outcomes, err := u.flow.submit(ctx, rec, transfers)
	if err != nil {
		return TransactionView{}, err
	}
	return u.settle(ctx, rec, req, event, legs, postings, reverts, classify(outcomes))
}

// resolvePostings returns the postings to apply: the request's, or the reversed postings of the
// transaction a revert names, read from TigerBeetle.
func (u *Transactions) resolvePostings(ctx context.Context, ledgerName string, req transactionRequest) ([]ledger.Posting, *ledger.ID, error) {
	if req.Reverts == "" {
		out := make([]ledger.Posting, 0, len(req.Postings))
		for _, p := range req.Postings {
			src, err := ledger.ParseAddress(p.Source)
			if err != nil {
				return nil, nil, err
			}
			dst, err := ledger.ParseAddress(p.Destination)
			if err != nil {
				return nil, nil, err
			}
			asset, err := ledger.ParseAsset(p.Asset)
			if err != nil {
				return nil, nil, err
			}
			amount, err := ledger.ParseAmount(p.Amount)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, ledger.Posting{Source: src, Destination: dst, Asset: asset, Amount: amount})
		}
		return out, nil, nil
	}
	id, err := ledger.ParseID(req.Reverts)
	if err != nil {
		return nil, nil, err
	}
	original, err := u.Get(ctx, ledgerName, id.String())
	if err != nil {
		return nil, nil, err
	}
	out := make([]ledger.Posting, 0, len(original.Postings))
	for i := len(original.Postings) - 1; i >= 0; i-- {
		p := original.Postings[i]
		asset, err := ledger.ParseAsset(p.Asset)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, ledger.Posting{Source: p.Destination, Destination: p.Source, Asset: asset, Amount: p.Amount})
	}
	return out, &id, nil
}

// failPrepared resolves the record as failed when the stored request can no longer be executed,
// for example an event type that does not exist. Errors that are not client errors leave the
// record pending.
func (u *Transactions) failPrepared(ctx context.Context, rec WriteRecord, err error) error {
	var p *ledger.Problem
	if !errors.As(err, &p) {
		if !errors.Is(err, ledger.ErrInvalid) && !errors.Is(err, ledger.ErrNotFound) {
			return err
		}
		p = ledger.NewProblem(ledger.ErrInvalid, "invalid-request", err.Error())
	}
	return u.flow.decline(ctx, rec, ledger.RequestFailed, p)
}

// settle resolves the record from the classified outcome of the chain.
func (u *Transactions) settle(ctx context.Context, rec WriteRecord, req transactionRequest, event ledger.EventType, legs []LegRow, postings []ledger.Posting, reverts *ledger.ID, v chainVerdict) (TransactionView, error) {
	switch v.verdict {
	case verdictCommitted:
		ts := v.timestamp
		if ts == 0 {
			var err error
			if ts, err = latestTimestamp(ctx, u.tb, rec.TransferIDs); err != nil {
				return TransactionView{}, err
			}
		}
		row := TransactionRow{
			ID:             rec.TargetID,
			Ledger:         rec.Ledger,
			Reference:      req.Reference,
			EventType:      event.Name,
			Code:           event.Code,
			BusinessTime:   req.BusinessTime,
			Metadata:       req.Metadata,
			IdempotencyKey: rec.Key,
			RequestID:      rec.RequestID,
			Reverts:        reverts,
			CommittedAt:    timeFromTB(ts),
			TBTimestamp:    ts,
			Legs:           legs,
		}
		if err := u.flow.writes.CommitTransaction(ctx, u.flow.committedOutcome(rec, statusCreated, rec.TargetID), row); err != nil {
			return TransactionView{}, fmt.Errorf("commit transaction: %w", err)
		}
		return transactionView(row, postings), nil
	case verdictBurned:
		return TransactionView{}, u.flow.decline(ctx, rec, ledger.RequestFailed, &ledger.Problem{
			Kind:    ledger.ErrRejected,
			Reason:  "transaction-declined",
			Detail:  "TigerBeetle declined this transaction on an earlier attempt and no longer reports why; nothing was applied. Retry with a new Idempotency-Key",
			Posting: -1,
			Results: v.results,
		})
	default:
		return TransactionView{}, u.flow.decline(ctx, rec, ledger.RequestDeclined, declineProblem(v, postings, legs))
	}
}

// declineProblem explains a declined chain in ledger terms. Only the failing posting has its own
// result; the others report linked_event_failed because the chain is atomic.
func declineProblem(v chainVerdict, postings []ledger.Posting, legs []LegRow) *ledger.Problem {
	p := &ledger.Problem{Kind: ledger.ErrRejected, Reason: resultReason(v.result), Posting: v.failed, Results: v.results}
	if v.failed < 0 || v.failed >= len(postings) {
		p.Reason = "transaction-declined"
		p.Detail = "TigerBeetle declined the chain; nothing was applied"
		return p
	}
	posting := postings[v.failed]
	n := v.failed + 1
	switch v.result {
	case ResultExceedsCredits:
		p.Detail = fmt.Sprintf("Posting %d would take %s below zero in %s. Nothing was applied: the postings are linked.", n, posting.Source, posting.Asset)
	case ResultExceedsDebits:
		p.Detail = fmt.Sprintf("Posting %d would take %s above its limit in %s (credits must not exceed debits). Nothing was applied: the postings are linked.", n, posting.Destination, posting.Asset)
	case ResultIDAlreadyFailed:
		p.Detail = fmt.Sprintf("Posting %d reuses a transfer ID that already failed. Nothing was applied.", n)
	default:
		p.Detail = fmt.Sprintf("Posting %d was declined by TigerBeetle with %s. Nothing was applied: the postings are linked.", n, v.result)
	}
	if v.failed < len(legs) && v.result == "debit_account_already_closed" {
		p.Detail = fmt.Sprintf("Posting %d debits %s, which is closed.", n, legs[v.failed].Source.Address)
	}
	return p
}

func transactionView(row TransactionRow, postings []ledger.Posting) TransactionView {
	v := TransactionView{
		ID:           row.ID,
		Ledger:       row.Ledger,
		Reference:    row.Reference,
		EventType:    ledger.EventType{Name: row.EventType, Code: row.Code, BuiltIn: row.EventType == ledger.DefaultEventType.Name},
		BusinessTime: row.BusinessTime,
		Metadata:     row.Metadata,
		// The TigerBeetle timestamp has nanoseconds; Postgres keeps microseconds.
		CommittedAt:    timeFromTB(row.TBTimestamp),
		Timestamp:      row.TBTimestamp,
		Reverts:        row.Reverts,
		RevertedBy:     row.RevertedBy,
		IdempotencyKey: row.IdempotencyKey,
		RequestID:      row.RequestID,
	}
	for i := range row.Legs {
		leg := &row.Legs[i]
		v.Postings = append(v.Postings, PostingView{
			Source:      leg.Source.Address,
			Destination: leg.Destination.Address,
			Asset:       leg.Source.Asset,
			Amount:      postings[i].Amount,
			TransferID:  leg.TransferID,
			TBLedger:    leg.Source.TBLedger,
		})
	}
	return v
}

// views reads the amounts of the transactions from TigerBeetle in one batch.
func (u *Transactions) views(ctx context.Context, rows []TransactionRow) ([]TransactionView, error) {
	var ids []ledger.ID
	for i := range rows {
		r := &rows[i]
		for i := range r.Legs {
			l := &r.Legs[i]
			ids = append(ids, l.TransferID)
		}
	}
	amounts := make(map[ledger.ID]ledger.Amount, len(ids))
	for start := 0; start < len(ids); start += LookupChunk {
		found, err := u.tb.LookupTransfers(ctx, ids[start:min(start+LookupChunk, len(ids))])
		if err != nil {
			return nil, fmt.Errorf("lookup transfers: %w", err)
		}
		for i := range found {
			t := &found[i]
			amounts[t.ID] = t.Amount
		}
	}
	out := make([]TransactionView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		postings := make([]ledger.Posting, len(r.Legs))
		for i := range r.Legs {
			l := &r.Legs[i]
			amount, ok := amounts[l.TransferID]
			if !ok {
				return nil, fmt.Errorf("%w: transfer %s of transaction %s is missing in TigerBeetle", ledger.ErrUnavailable, l.TransferID, r.ID)
			}
			postings[i] = ledger.Posting{Amount: amount}
		}
		out = append(out, transactionView(*r, postings))
	}
	return out, nil
}

// Get returns a committed transaction with its amounts read from TigerBeetle.
func (u *Transactions) Get(ctx context.Context, ledgerName, id string) (TransactionView, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return TransactionView{}, err
	}
	txID, err := ledger.ParseID(id)
	if err != nil {
		return TransactionView{}, err
	}
	row, err := u.txs.GetTransaction(ctx, ledgerName, txID)
	if err != nil {
		return TransactionView{}, fmt.Errorf("get transaction %s: %w", txID, err)
	}
	views, err := u.views(ctx, []TransactionRow{row})
	if err != nil {
		return TransactionView{}, err
	}
	return views[0], nil
}

// ListInput is the transaction list query.
type ListInput struct {
	Ledger    string
	Address   string
	Asset     string
	Reference string
	EventType string
	After     *time.Time
	Before    *time.Time
	Cursor    string
	Limit     int
}

// List returns committed transactions newest first.
func (u *Transactions) List(ctx context.Context, in ListInput) ([]TransactionView, string, error) {
	if err := requireLedger(ctx, u.ledgers, in.Ledger); err != nil {
		return nil, "", err
	}
	filter := TransactionFilter{Reference: in.Reference, EventType: in.EventType, After: in.After, Before: in.Before}
	p, err := parsePrefix(in.Address)
	if err != nil {
		return nil, "", err
	}
	filter.Address = p
	if in.Asset != "" {
		a, err := ledger.ParseAsset(in.Asset)
		if err != nil {
			return nil, "", err
		}
		filter.Asset = a.String()
	}
	before, err := decodeIDCursor(in.Cursor)
	if err != nil {
		return nil, "", err
	}
	limit := pageSize(in.Limit)
	rows, err := u.txs.ListTransactions(ctx, in.Ledger, filter, before, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list transactions: %w", err)
	}
	page, next := trimPage(rows, limit, func(r TransactionRow) string { return r.ID.String() })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", err
	}
	return views, next, nil
}

// ReplaceMetadata replaces the metadata stored with a transaction; the postings never change.
func (u *Transactions) ReplaceMetadata(ctx context.Context, ledgerName, id string, metadata map[string]string) (TransactionView, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return TransactionView{}, err
	}
	txID, err := ledger.ParseID(id)
	if err != nil {
		return TransactionView{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return TransactionView{}, err
	}
	if err := u.txs.ReplaceTransactionMetadata(ctx, ledgerName, txID, metadata); err != nil {
		return TransactionView{}, fmt.Errorf("replace metadata of %s: %w", txID, err)
	}
	return u.Get(ctx, ledgerName, id)
}

// settlePending is the reconciler path for a transaction or revert record (ADR-0007): look up the
// stored IDs; all found means the chain landed, none found means it never reached the cluster and
// is resubmitted with the same IDs, some found is a bug and stays pending.
func (u *Transactions) settlePending(ctx context.Context, rec WriteRecord) (bool, error) {
	var req transactionRequest
	if err := json.Unmarshal(rec.Request, &req); err != nil {
		return false, fmt.Errorf("decode stored request of %s: %w", rec.Key, err)
	}
	found, err := u.tb.LookupTransfers(ctx, rec.TransferIDs)
	if err != nil {
		return false, fmt.Errorf("lookup transfers: %w", err)
	}
	switch {
	case len(found) == 0:
		_, err := u.execute(ctx, rec, req)
		var p *ledger.Problem
		if err != nil && errors.As(err, &p) && (errors.Is(p, ledger.ErrRejected) || errors.Is(p, ledger.ErrInvalid)) {
			// Declined and recorded: settled.
			return true, nil
		}
		return err == nil, err
	case len(found) == len(rec.TransferIDs):
		postings, reverts, err := u.resolvePostings(ctx, rec.Ledger, req)
		if err != nil {
			return false, err
		}
		event, err := resolveEventType(ctx, u.events, rec.Ledger, req.EventType)
		if err != nil {
			return false, err
		}
		keys := make([]AccountKey, 0, 2*len(postings))
		for _, p := range postings {
			keys = append(keys, AccountKey{Address: p.Source, Asset: p.Asset.String()}, AccountKey{Address: p.Destination, Asset: p.Asset.String()})
		}
		rows, err := u.accounts.FindAccounts(ctx, rec.Ledger, keys)
		if err != nil {
			return false, err
		}
		byKey := make(map[AccountKey]AccountRow, len(rows))
		for i := range rows {
			r := &rows[i]
			byKey[r.Key()] = *r
		}
		legs := make([]LegRow, len(postings))
		var ts uint64
		for i := range found {
			t := &found[i]
			ts = max(ts, t.Timestamp)
		}
		for i, p := range postings {
			legs[i] = LegRow{
				TransferID:  rec.TransferIDs[i],
				Source:      byKey[AccountKey{Address: p.Source, Asset: p.Asset.String()}],
				Destination: byKey[AccountKey{Address: p.Destination, Asset: p.Asset.String()}],
			}
		}
		_, err = u.settle(ctx, rec, req, event, legs, postings, reverts, chainVerdict{verdict: verdictCommitted, timestamp: ts, failed: -1})
		return err == nil, err
	default:
		u.logger.ErrorContext(ctx, "linked chain partially present in TigerBeetle; a chain cannot produce this state",
			slog.String("ledger", rec.Ledger), slog.String("idempotency_key", rec.Key), slog.Int("found", len(found)), slog.Int("expected", len(rec.TransferIDs)))
		return false, nil
	}
}
