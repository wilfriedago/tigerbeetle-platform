package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

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

type transactionCommands struct {
	ledgers  LedgerDirectory
	events   EventTypes
	txs      domain.TransactionRepository
	accounts *accountEnsurer
	flow     *writeFlow
	tb       domain.LedgerStore
	ids      domain.IDGenerator
	clock    ledger.Clock
	logger   *slog.Logger
	queries  *transactionQueries
}

func (u *transactionCommands) Create(ctx context.Context, in CreateTransactionInput) (TransactionView, error) {
	if err := domain.ValidateIdempotencyKey(in.Key); err != nil {
		return TransactionView{}, err
	}
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return TransactionView{}, err
	}
	if err := domain.ValidatePostings(in.Postings); err != nil {
		return TransactionView{}, err
	}
	if err := domain.ValidateReference(in.Reference); err != nil {
		return TransactionView{}, err
	}
	if in.Metadata == nil {
		in.Metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(in.Metadata); err != nil {
		return TransactionView{}, err
	}
	event, err := u.events.Resolve(ctx, in.Ledger, in.EventType)
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
	return u.write(ctx, in.Ledger, in.Key, in.RequestID, domain.OperationTransaction, req, len(in.Postings))
}

func (u *transactionCommands) Revert(ctx context.Context, in RevertTransactionInput) (TransactionView, error) {
	if err := domain.ValidateIdempotencyKey(in.Key); err != nil {
		return TransactionView{}, err
	}
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return TransactionView{}, err
	}
	txID, err := ledger.ParseID(in.ID)
	if err != nil {
		return TransactionView{}, err
	}
	if err := domain.ValidateReference(in.Reference); err != nil {
		return TransactionView{}, err
	}
	metadata := in.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return TransactionView{}, err
	}
	original, err := u.txs.GetTransaction(ctx, in.Ledger, txID)
	if err != nil {
		return TransactionView{}, fmt.Errorf("get transaction %s: %w", txID, err)
	}
	req := transactionRequest{Reverts: txID.String(), Reference: in.Reference, EventType: original.EventType, Metadata: metadata}
	// A key that already holds this revert replays it, even though the original now shows as
	// reverted; only a revert under a different key is refused.
	if original.RevertedBy != nil {
		if rec, err := u.flow.writes.GetWrite(ctx, in.Ledger, in.Key); err != nil || rec.Operation != domain.OperationRevert {
			return TransactionView{}, ledger.NewProblem(ledger.ErrConflict, "already-reverted",
				fmt.Sprintf("transaction %s was already reverted by %s; a transaction is reverted at most once", txID, original.RevertedBy))
		}
	}
	return u.write(ctx, in.Ledger, in.Key, in.RequestID, domain.OperationRevert, req, len(original.Legs))
}

func (u *transactionCommands) ReplaceMetadata(ctx context.Context, ledgerName, id string, metadata map[string]string) (TransactionView, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
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
	return u.queries.Get(ctx, ledgerName, id)
}

// write records the request with minted IDs, then executes it, or answers from the record.
func (u *transactionCommands) write(ctx context.Context, ledgerName, key, requestID string, op domain.Operation, req transactionRequest, legs int) (TransactionView, error) {
	body, hash, err := canonicalRequest(req)
	if err != nil {
		return TransactionView{}, err
	}
	rec := domain.WriteRecord{
		Ledger:      ledgerName,
		Key:         key,
		Operation:   op,
		Status:      domain.RequestPending,
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
		if errors.Is(err, ledger.ErrConflict) && op == domain.OperationRevert && !errors.As(err, new(*ledger.Problem)) {
			return TransactionView{}, ledger.NewProblem(ledger.ErrConflict, "already-reverted",
				fmt.Sprintf("transaction %s is already being reverted under another Idempotency-Key", req.Reverts))
		}
		return TransactionView{}, err
	}
	if !fresh {
		return u.queries.Get(ctx, ledgerName, stored.TargetID.String())
	}
	return u.execute(ctx, stored, req)
}

// execute builds the chain from the record and submits it, then resolves the record. The
// reconciler calls it again with the same record when nothing reached the cluster.
func (u *transactionCommands) execute(ctx context.Context, rec domain.WriteRecord, req transactionRequest) (TransactionView, error) {
	postings, reverts, err := u.resolvePostings(ctx, rec.Ledger, req)
	if err != nil {
		return TransactionView{}, u.failPrepared(ctx, rec, err)
	}
	if len(postings) != len(rec.TransferIDs) {
		return TransactionView{}, fmt.Errorf("record %s has %d transfer IDs for %d postings", rec.Key, len(rec.TransferIDs), len(postings))
	}
	event, err := u.events.Resolve(ctx, rec.Ledger, req.EventType)
	if err != nil {
		return TransactionView{}, u.failPrepared(ctx, rec, err)
	}
	keys := make([]domain.AccountKey, 0, 2*len(postings))
	for _, p := range postings {
		keys = append(keys, domain.AccountKey{Address: p.Source, Asset: p.Asset.String()}, domain.AccountKey{Address: p.Destination, Asset: p.Asset.String()})
	}
	accounts, err := u.accounts.ensure(ctx, rec.Ledger, keys)
	if err != nil {
		return TransactionView{}, u.flow.pending(ctx, rec, err)
	}
	var businessMillis uint64
	if req.BusinessTime != nil {
		businessMillis = uint64(req.BusinessTime.UnixMilli())
	}
	transfers := make([]domain.TransferSpec, len(postings))
	legs := make([]domain.LegRow, len(postings))
	for i, p := range postings {
		src := accounts[domain.AccountKey{Address: p.Source, Asset: p.Asset.String()}]
		dst := accounts[domain.AccountKey{Address: p.Destination, Asset: p.Asset.String()}]
		transfers[i] = domain.TransferSpec{
			ID:              rec.TransferIDs[i],
			DebitAccountID:  src.ID,
			CreditAccountID: dst.ID,
			Amount:          p.Amount,
			TBLedger:        src.TBLedger,
			Code:            event.Code,
			Kind:            domain.TransferPosting,
			// Every transfer but the last carries linked; a one-transfer chain sets no flag, which
			// would otherwise fail with linked_event_chain_open (ADR-0007).
			Linked:      i < len(postings)-1,
			UserData128: rec.TargetID,
			UserData64:  businessMillis,
		}
		legs[i] = domain.LegRow{TransferID: rec.TransferIDs[i], Source: src, Destination: dst}
	}
	outcomes, err := u.flow.submit(ctx, rec, transfers)
	if err != nil {
		return TransactionView{}, err
	}
	return u.settle(ctx, rec, req, event, legs, postings, reverts, domain.Classify(outcomes))
}

// resolvePostings returns the postings to apply: the request's, or the reversed postings of the
// transaction a revert names, read from TigerBeetle.
func (u *transactionCommands) resolvePostings(ctx context.Context, ledgerName string, req transactionRequest) ([]domain.Posting, *ledger.ID, error) {
	if req.Reverts == "" {
		out := make([]domain.Posting, 0, len(req.Postings))
		for _, p := range req.Postings {
			posting, err := parsePosting(p.Source, p.Destination, p.Asset, p.Amount)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, posting)
		}
		return out, nil, nil
	}
	id, err := ledger.ParseID(req.Reverts)
	if err != nil {
		return nil, nil, err
	}
	original, err := u.queries.Get(ctx, ledgerName, id.String())
	if err != nil {
		return nil, nil, err
	}
	out := make([]domain.Posting, 0, len(original.Postings))
	for i := len(original.Postings) - 1; i >= 0; i-- {
		p := original.Postings[i]
		asset, err := ledger.ParseAsset(p.Asset)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, domain.Posting{Source: p.Destination, Destination: p.Source, Asset: asset, Amount: p.Amount})
	}
	return out, &id, nil
}

// parsePosting reads a posting stored on a record.
func parsePosting(source, destination, asset, amount string) (domain.Posting, error) {
	src, err := ledger.ParseAddress(source)
	if err != nil {
		return domain.Posting{}, err
	}
	dst, err := ledger.ParseAddress(destination)
	if err != nil {
		return domain.Posting{}, err
	}
	a, err := ledger.ParseAsset(asset)
	if err != nil {
		return domain.Posting{}, err
	}
	amt, err := ledger.ParseAmount(amount)
	if err != nil {
		return domain.Posting{}, err
	}
	return domain.Posting{Source: src, Destination: dst, Asset: a, Amount: amt}, nil
}

// failPrepared resolves the record as failed when the stored request can no longer be executed,
// for example an event type that does not exist. Errors that are not client errors leave the
// record pending.
func (u *transactionCommands) failPrepared(ctx context.Context, rec domain.WriteRecord, err error) error {
	var p *ledger.Problem
	if !errors.As(err, &p) {
		if !errors.Is(err, ledger.ErrInvalid) && !errors.Is(err, ledger.ErrNotFound) {
			return err
		}
		p = ledger.NewProblem(ledger.ErrInvalid, "invalid-request", err.Error())
	}
	return u.flow.decline(ctx, rec, domain.RequestFailed, p)
}

// settle resolves the record from the classified outcome of the chain.
func (u *transactionCommands) settle(ctx context.Context, rec domain.WriteRecord, req transactionRequest, event ledger.EventType,
	legs []domain.LegRow, postings []domain.Posting, reverts *ledger.ID, v domain.ChainVerdict,
) (TransactionView, error) {
	switch v.Verdict {
	case domain.VerdictCommitted:
		ts := v.Timestamp
		if ts == 0 {
			var err error
			if ts, err = latestTimestamp(ctx, u.tb, rec.TransferIDs); err != nil {
				return TransactionView{}, err
			}
		}
		row := domain.TransactionRow{
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
			CommittedAt:    domain.TimeFromTB(ts),
			TBTimestamp:    ts,
			Legs:           legs,
		}
		if err := u.flow.writes.CommitTransaction(ctx, u.flow.committedOutcome(rec, domain.StatusCreated, rec.TargetID), row); err != nil {
			return TransactionView{}, fmt.Errorf("commit transaction: %w", err)
		}
		return transactionView(row, postings), nil
	case domain.VerdictBurned:
		return TransactionView{}, u.flow.decline(ctx, rec, domain.RequestFailed, &ledger.Problem{
			Kind:    ledger.ErrRejected,
			Reason:  "transaction-declined",
			Detail:  "TigerBeetle declined this transaction on an earlier attempt and no longer reports why; nothing was applied. Retry with a new Idempotency-Key",
			Posting: -1,
			Results: v.Results,
		})
	default:
		return TransactionView{}, u.flow.decline(ctx, rec, domain.RequestDeclined, declineProblem(v, postings, legs))
	}
}

// declineProblem explains a declined chain in ledger terms. Only the failing posting has its own
// result; the others report linked_event_failed because the chain is atomic.
func declineProblem(v domain.ChainVerdict, postings []domain.Posting, legs []domain.LegRow) *ledger.Problem {
	p := &ledger.Problem{Kind: ledger.ErrRejected, Reason: domain.ResultReason(v.Result), Posting: v.Failed, Results: v.Results}
	if v.Failed < 0 || v.Failed >= len(postings) {
		p.Reason = "transaction-declined"
		p.Detail = "TigerBeetle declined the chain; nothing was applied"
		return p
	}
	posting := postings[v.Failed]
	n := v.Failed + 1
	switch v.Result {
	case domain.ResultExceedsCredits:
		p.Detail = fmt.Sprintf("Posting %d would take %s below zero in %s. Nothing was applied: the postings are linked.", n, posting.Source, posting.Asset)
	case domain.ResultExceedsDebits:
		p.Detail = fmt.Sprintf("Posting %d would take %s above its limit in %s (credits must not exceed debits). Nothing was applied: the postings are linked.", n, posting.Destination, posting.Asset)
	case domain.ResultIDAlreadyFailed:
		p.Detail = fmt.Sprintf("Posting %d reuses a transfer ID that already failed. Nothing was applied.", n)
	default:
		p.Detail = fmt.Sprintf("Posting %d was declined by TigerBeetle with %s. Nothing was applied: the postings are linked.", n, v.Result)
	}
	if v.Failed < len(legs) && v.Result == "debit_account_already_closed" {
		p.Detail = fmt.Sprintf("Posting %d debits %s, which is closed.", n, legs[v.Failed].Source.Address)
	}
	return p
}

// settlePending is the reconciler path for a transaction or revert record (ADR-0007): look up the
// stored IDs; all found means the chain landed, none found means it never reached the cluster and
// is resubmitted with the same IDs, some found is a bug and stays pending.
func (u *transactionCommands) settlePending(ctx context.Context, rec domain.WriteRecord) (bool, error) {
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
		event, err := u.events.Resolve(ctx, rec.Ledger, req.EventType)
		if err != nil {
			return false, err
		}
		keys := make([]domain.AccountKey, 0, 2*len(postings))
		for _, p := range postings {
			keys = append(keys, domain.AccountKey{Address: p.Source, Asset: p.Asset.String()}, domain.AccountKey{Address: p.Destination, Asset: p.Asset.String()})
		}
		rows, err := u.accounts.accounts.FindAccounts(ctx, rec.Ledger, keys)
		if err != nil {
			return false, err
		}
		byKey := make(map[domain.AccountKey]domain.AccountRow, len(rows))
		for i := range rows {
			byKey[rows[i].Key()] = rows[i]
		}
		legs := make([]domain.LegRow, len(postings))
		var ts uint64
		for i := range found {
			ts = max(ts, found[i].Timestamp)
		}
		for i, p := range postings {
			legs[i] = domain.LegRow{
				TransferID:  rec.TransferIDs[i],
				Source:      byKey[domain.AccountKey{Address: p.Source, Asset: p.Asset.String()}],
				Destination: byKey[domain.AccountKey{Address: p.Destination, Asset: p.Asset.String()}],
			}
		}
		_, err = u.settle(ctx, rec, req, event, legs, postings, reverts, domain.ChainVerdict{Verdict: domain.VerdictCommitted, Timestamp: ts, Failed: -1})
		return err == nil, err
	default:
		u.logger.ErrorContext(ctx, "linked chain partially present in TigerBeetle; a chain cannot produce this state",
			slog.String("ledger", rec.Ledger), slog.String("idempotency_key", rec.Key), slog.Int("found", len(found)), slog.Int("expected", len(rec.TransferIDs)))
		return false, nil
	}
}
