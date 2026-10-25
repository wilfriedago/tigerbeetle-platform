package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// HoldView is a hold with its amounts read from TigerBeetle (ADR-0010).
type HoldView struct {
	ID                   ledger.ID
	Ledger               string
	Source               ledger.Address
	Destination          ledger.Address
	Asset                string
	Amount               ledger.Amount
	CapturedAmount       *ledger.Amount
	Status               ledger.HoldStatus
	TimeoutSeconds       uint32
	CreatedAt            time.Time
	Timestamp            uint64
	ExpiresAt            *time.Time
	ResolvedAt           *time.Time
	Reference            string
	EventType            ledger.EventType
	Metadata             map[string]string
	IdempotencyKey       string
	PendingTransferID    ledger.ID
	ResolutionTransferID *ledger.ID
	TBLedger             uint32
}

// CreateHoldInput is a hold request after HTTP decoding.
type CreateHoldInput struct {
	Ledger         string
	Key            string
	RequestID      string
	Posting        ledger.Posting
	TimeoutSeconds uint32
	Reference      string
	EventType      string
	Metadata       map[string]string
}

type holdRequest struct {
	Source         string            `json:"source"`
	Destination    string            `json:"destination"`
	Asset          string            `json:"asset"`
	Amount         string            `json:"amount"`
	TimeoutSeconds uint32            `json:"timeoutSeconds"`
	Reference      string            `json:"reference,omitempty"`
	EventType      string            `json:"eventType,omitempty"`
	Metadata       map[string]string `json:"metadata"`
}

// resolutionRequest is the stored body of a capture or void. Hold is read by the unique index
// write_requests_one_resolution, so a hold resolves once whatever the keys.
type resolutionRequest struct {
	Hold   string `json:"hold"`
	Amount string `json:"amount,omitempty"`
}

// Holds runs the two-phase transfer flows of ADR-0010 on the write path of ADR-0007.
type Holds struct {
	ledgers  LedgerRepository
	events   EventTypeRepository
	rules    RuleRepository
	accounts AccountRepository
	holds    HoldRepository
	flow     *writeFlow
	tb       LedgerStore
	ids      IDGenerator
	clock    Clock
	logger   *slog.Logger
}

// NewHolds builds the use case. It takes the same dependencies as transactions.
func NewHolds(d TransactionsDeps) *Holds {
	grace := d.GracePeriod
	if grace <= 0 {
		grace = DefaultGracePeriod
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Holds{
		ledgers:  d.Store,
		events:   d.Store,
		rules:    d.Store,
		accounts: d.Store,
		holds:    d.Store,
		flow:     &writeFlow{writes: d.Store, tb: d.TigerBeetle, clock: d.Clock, grace: grace, logger: logger},
		tb:       d.TigerBeetle,
		ids:      d.IDs,
		clock:    d.Clock,
		logger:   logger,
	}
}

// Create reserves the amount with a pending transfer. The source's available balance drops at
// once, because debits_must_not_exceed_credits counts pending debits.
func (u *Holds) Create(ctx context.Context, in CreateHoldInput) (HoldView, error) {
	if err := ledger.ValidateIdempotencyKey(in.Key); err != nil {
		return HoldView{}, err
	}
	if err := requireLedger(ctx, u.ledgers, in.Ledger); err != nil {
		return HoldView{}, err
	}
	if err := in.Posting.Validate(0); err != nil {
		return HoldView{}, err
	}
	if err := ledger.ValidateReference(in.Reference); err != nil {
		return HoldView{}, err
	}
	if in.Metadata == nil {
		in.Metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(in.Metadata); err != nil {
		return HoldView{}, err
	}
	event, err := resolveEventType(ctx, u.events, in.Ledger, in.EventType)
	if err != nil {
		return HoldView{}, err
	}
	req := holdRequest{
		Source:         string(in.Posting.Source),
		Destination:    string(in.Posting.Destination),
		Asset:          in.Posting.Asset.String(),
		Amount:         in.Posting.Amount.String(),
		TimeoutSeconds: in.TimeoutSeconds,
		Reference:      in.Reference,
		EventType:      event.Name,
		Metadata:       in.Metadata,
	}
	body, hash, err := canonicalRequest(req)
	if err != nil {
		return HoldView{}, err
	}
	rec := WriteRecord{
		Ledger:      in.Ledger,
		Key:         in.Key,
		Operation:   ledger.OperationHold,
		Status:      ledger.RequestPending,
		BodyHash:    hash,
		Request:     body,
		TargetID:    u.ids.NewID(),
		TransferIDs: []ledger.ID{u.ids.NewID()},
		RequestID:   in.RequestID,
		CreatedAt:   u.clock.Now().UTC(),
	}
	stored, fresh, err := u.flow.begin(ctx, rec)
	if err != nil {
		return HoldView{}, err
	}
	if !fresh {
		return u.Get(ctx, in.Ledger, stored.TargetID.String())
	}
	return u.executeCreate(ctx, stored, req)
}

func (u *Holds) executeCreate(ctx context.Context, rec WriteRecord, req holdRequest) (HoldView, error) {
	posting, err := parseHoldPosting(req)
	if err != nil {
		return HoldView{}, u.flow.decline(ctx, rec, ledger.RequestFailed, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", err.Error()))
	}
	event, err := resolveEventType(ctx, u.events, rec.Ledger, req.EventType)
	if err != nil {
		var p *ledger.Problem
		if errors.As(err, &p) {
			return HoldView{}, u.flow.decline(ctx, rec, ledger.RequestFailed, p)
		}
		return HoldView{}, err
	}
	src, dst, err := u.holdAccounts(ctx, rec.Ledger, posting)
	if err != nil {
		return HoldView{}, u.flow.pending(ctx, rec, err)
	}
	transfer := TransferSpec{
		ID:              rec.TransferIDs[0],
		DebitAccountID:  src.ID,
		CreditAccountID: dst.ID,
		Amount:          posting.Amount,
		TBLedger:        src.TBLedger,
		Code:            event.Code,
		Kind:            TransferPending,
		TimeoutSeconds:  req.TimeoutSeconds,
		UserData128:     rec.TargetID,
	}
	outcomes, err := u.flow.submit(ctx, rec, []TransferSpec{transfer})
	if err != nil {
		return HoldView{}, err
	}
	return u.settleCreate(ctx, rec, req, event, src, dst, posting, classify(outcomes))
}

func (u *Holds) holdAccounts(ctx context.Context, ledgerName string, posting ledger.Posting) (src, dst AccountRow, err error) {
	srcKey := AccountKey{Address: posting.Source, Asset: posting.Asset.String()}
	dstKey := AccountKey{Address: posting.Destination, Asset: posting.Asset.String()}
	accounts, err := ensureAccounts(ctx, u.accounts, u.rules, u.tb, u.ids, u.clock, ledgerName, []AccountKey{srcKey, dstKey})
	if err != nil {
		return AccountRow{}, AccountRow{}, err
	}
	return accounts[srcKey], accounts[dstKey], nil
}

func parseHoldPosting(req holdRequest) (ledger.Posting, error) {
	src, err := ledger.ParseAddress(req.Source)
	if err != nil {
		return ledger.Posting{}, err
	}
	dst, err := ledger.ParseAddress(req.Destination)
	if err != nil {
		return ledger.Posting{}, err
	}
	asset, err := ledger.ParseAsset(req.Asset)
	if err != nil {
		return ledger.Posting{}, err
	}
	amount, err := ledger.ParseAmount(req.Amount)
	if err != nil {
		return ledger.Posting{}, err
	}
	return ledger.Posting{Source: src, Destination: dst, Asset: asset, Amount: amount}, nil
}

func (u *Holds) settleCreate(ctx context.Context, rec WriteRecord, req holdRequest, event ledger.EventType, src, dst AccountRow, posting ledger.Posting, v chainVerdict) (HoldView, error) {
	switch v.verdict {
	case verdictCommitted:
		ts := v.timestamp
		if ts == 0 {
			var err error
			if ts, err = latestTimestamp(ctx, u.tb, rec.TransferIDs); err != nil {
				return HoldView{}, err
			}
		}
		row := HoldRow{
			ID:                rec.TargetID,
			Ledger:            rec.Ledger,
			PendingTransferID: rec.TransferIDs[0],
			Source:            src,
			Destination:       dst,
			Reference:         req.Reference,
			EventType:         event.Name,
			Code:              event.Code,
			Metadata:          req.Metadata,
			TimeoutSeconds:    req.TimeoutSeconds,
			IdempotencyKey:    rec.Key,
			RequestID:         rec.RequestID,
			CreatedAt:         timeFromTB(ts),
			TBTimestamp:       ts,
			Status:            ledger.HoldOpen,
		}
		if err := u.flow.writes.CommitHold(ctx, u.flow.committedOutcome(rec, statusCreated, rec.TargetID), row); err != nil {
			return HoldView{}, fmt.Errorf("commit hold: %w", err)
		}
		return u.view(row, posting.Amount, nil), nil
	case verdictBurned:
		return HoldView{}, u.flow.decline(ctx, rec, ledger.RequestFailed, &ledger.Problem{
			Kind: ledger.ErrRejected, Reason: "hold-declined", Posting: -1, Results: v.results,
			Detail: "TigerBeetle declined this hold on an earlier attempt and no longer reports why; nothing was reserved. Retry with a new Idempotency-Key",
		})
	default:
		p := &ledger.Problem{Kind: ledger.ErrRejected, Reason: resultReason(v.result), Posting: -1, Results: v.results}
		switch v.result {
		case ResultExceedsCredits:
			p.Detail = fmt.Sprintf("The hold would take %s below zero in %s: its available balance, which already counts open holds, is lower than the amount.", posting.Source, posting.Asset)
		case ResultExceedsDebits:
			p.Detail = fmt.Sprintf("The hold would take %s above its limit in %s (credits must not exceed debits).", posting.Destination, posting.Asset)
		default:
			p.Detail = fmt.Sprintf("TigerBeetle declined the hold with %s; nothing was reserved.", v.result)
		}
		return HoldView{}, u.flow.decline(ctx, rec, ledger.RequestDeclined, p)
	}
}

// Capture posts the pending transfer: the full amount, or a smaller amount with the rest released.
// A hold resolves exactly once (ADR-0010).
func (u *Holds) Capture(ctx context.Context, ledgerName, key, requestID, id string, amount *ledger.Amount) (HoldView, error) {
	req := resolutionRequest{Hold: id}
	if amount != nil {
		if amount.IsZero() {
			// TigerBeetle would read a zero post as "void the rest", turning a capture into a void.
			return HoldView{}, fmt.Errorf("%w: capture amount must be greater than zero; void the hold to release it", ledger.ErrInvalid)
		}
		req.Amount = amount.String()
	}
	return u.resolve(ctx, ledgerName, key, requestID, ledger.OperationCapture, req)
}

// Void releases the whole hold back to the source.
func (u *Holds) Void(ctx context.Context, ledgerName, key, requestID, id string) (HoldView, error) {
	return u.resolve(ctx, ledgerName, key, requestID, ledger.OperationVoid, resolutionRequest{Hold: id})
}

func (u *Holds) resolve(ctx context.Context, ledgerName, key, requestID string, op ledger.Operation, req resolutionRequest) (HoldView, error) {
	if err := ledger.ValidateIdempotencyKey(key); err != nil {
		return HoldView{}, err
	}
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return HoldView{}, err
	}
	holdID, err := ledger.ParseID(req.Hold)
	if err != nil {
		return HoldView{}, err
	}
	hold, err := u.holds.GetHold(ctx, ledgerName, holdID)
	if err != nil {
		return HoldView{}, fmt.Errorf("get hold %s: %w", holdID, err)
	}
	body, hash, err := canonicalRequest(req)
	if err != nil {
		return HoldView{}, err
	}
	rec := WriteRecord{
		Ledger:      ledgerName,
		Key:         key,
		Operation:   op,
		Status:      ledger.RequestPending,
		BodyHash:    hash,
		Request:     body,
		TargetID:    holdID,
		TransferIDs: []ledger.ID{u.ids.NewID()},
		RequestID:   requestID,
		CreatedAt:   u.clock.Now().UTC(),
	}
	if hold.Status == ledger.HoldCaptured || hold.Status == ledger.HoldVoided {
		// A replay of the write that resolved the hold answers with the hold; any other write is
		// refused before minting anything.
		if existing, err := u.flow.writes.GetWrite(ctx, ledgerName, key); err == nil && existing.Operation == op && bytes.Equal(existing.BodyHash, hash) {
			return u.Get(ctx, ledgerName, holdID.String())
		}
		return HoldView{}, alreadyResolved(hold)
	}
	stored, fresh, err := u.flow.begin(ctx, rec)
	if err != nil {
		if errors.Is(err, ledger.ErrConflict) && !errors.As(err, new(*ledger.Problem)) {
			return HoldView{}, ledger.NewProblem(ledger.ErrConflict, "hold-already-resolved", fmt.Sprintf("hold %s is already being captured or voided under another Idempotency-Key", holdID))
		}
		return HoldView{}, err
	}
	if !fresh {
		return u.Get(ctx, ledgerName, holdID.String())
	}
	return u.executeResolve(ctx, stored, req, hold)
}

func alreadyResolved(hold HoldRow) error {
	return ledger.NewProblem(ledger.ErrConflict, "hold-already-resolved", fmt.Sprintf("hold %s is already %s; a hold is captured or voided exactly once", hold.ID, hold.Status))
}

func (u *Holds) executeResolve(ctx context.Context, rec WriteRecord, req resolutionRequest, hold HoldRow) (HoldView, error) {
	transfer := TransferSpec{
		ID:              rec.TransferIDs[0],
		PendingID:       hold.PendingTransferID,
		DebitAccountID:  hold.Source.ID,
		CreditAccountID: hold.Destination.ID,
		TBLedger:        hold.Source.TBLedger,
		// A post or void must carry the pending transfer's code or zero; repeating it keeps every
		// transfer self-describing (ADR-0010).
		Code:        hold.Code,
		UserData128: hold.ID,
	}
	if rec.Operation == ledger.OperationCapture {
		transfer.Kind = TransferPostPending
		transfer.Amount = ledger.MaxAmount
		if req.Amount != "" {
			amount, err := ledger.ParseAmount(req.Amount)
			if err != nil {
				return HoldView{}, err
			}
			transfer.Amount = amount
		}
	} else {
		transfer.Kind = TransferVoidPending
	}
	outcomes, err := u.flow.submit(ctx, rec, []TransferSpec{transfer})
	if err != nil {
		return HoldView{}, err
	}
	return u.settleResolve(ctx, rec, hold, classify(outcomes))
}

func (u *Holds) settleResolve(ctx context.Context, rec WriteRecord, hold HoldRow, v chainVerdict) (HoldView, error) {
	status := ledger.HoldCaptured
	if rec.Operation == ledger.OperationVoid {
		status = ledger.HoldVoided
	}
	switch v.verdict {
	case verdictCommitted:
		ts := v.timestamp
		if ts == 0 {
			var err error
			if ts, err = latestTimestamp(ctx, u.tb, rec.TransferIDs); err != nil {
				return HoldView{}, err
			}
		}
		resolution := HoldResolution{Ledger: rec.Ledger, HoldID: hold.ID, Status: status, ResolutionTransferID: rec.TransferIDs[0], ResolvedAt: timeFromTB(ts)}
		if err := u.flow.writes.CommitResolution(ctx, u.flow.committedOutcome(rec, statusOK, hold.ID), resolution); err != nil {
			return HoldView{}, fmt.Errorf("commit %s: %w", rec.Operation, err)
		}
		return u.Get(ctx, rec.Ledger, hold.ID.String())
	case verdictBurned:
		return HoldView{}, u.flow.decline(ctx, rec, ledger.RequestFailed, &ledger.Problem{
			Kind: ledger.ErrRejected, Reason: "hold-resolution-declined", Posting: -1, Results: v.results,
			Detail: "TigerBeetle declined this write on an earlier attempt and no longer reports why. Retry with a new Idempotency-Key",
		})
	case verdictDeclined:
		// Explained below, per TigerBeetle result.
	}
	p := &ledger.Problem{Kind: ledger.ErrRejected, Reason: resultReason(v.result), Posting: -1, Results: v.results}
	switch v.result {
	case ResultPendingTransferExpired:
		if err := u.holds.MarkHoldExpired(ctx, rec.Ledger, hold.ID); err != nil {
			return HoldView{}, fmt.Errorf("mark hold expired: %w", err)
		}
		p.Kind, p.Reason = ledger.ErrConflict, "hold-expired"
		p.Detail = fmt.Sprintf("hold %s expired on the cluster clock and TigerBeetle released it; it can no longer be captured or voided", hold.ID)
	case ResultPendingTransferAlreadyPosted, ResultPendingTransferAlreadyVoided:
		p.Kind, p.Reason = ledger.ErrConflict, "hold-already-resolved"
		p.Detail = fmt.Sprintf("hold %s was already resolved; a hold is captured or voided exactly once", hold.ID)
	case ResultExceedsPendingTransferAmount:
		p.Kind, p.Reason = ledger.ErrInvalid, "capture-exceeds-hold"
		p.Detail = "the capture amount is larger than the hold"
	default:
		p.Detail = fmt.Sprintf("TigerBeetle declined the %s with %s", rec.Operation, v.result)
	}
	return HoldView{}, u.flow.decline(ctx, rec, ledger.RequestDeclined, p)
}

func (u *Holds) view(row HoldRow, amount ledger.Amount, captured *ledger.Amount) HoldView {
	v := HoldView{
		ID:                   row.ID,
		Ledger:               row.Ledger,
		Source:               row.Source.Address,
		Destination:          row.Destination.Address,
		Asset:                row.Source.Asset,
		Amount:               amount,
		CapturedAmount:       captured,
		Status:               ledger.HoldStatusAt(row.Status, row.CreatedAt, row.TimeoutSeconds, u.clock.Now()),
		TimeoutSeconds:       row.TimeoutSeconds,
		CreatedAt:            timeFromTB(row.TBTimestamp),
		Timestamp:            row.TBTimestamp,
		ResolvedAt:           row.ResolvedAt,
		Reference:            row.Reference,
		EventType:            ledger.EventType{Name: row.EventType, Code: row.Code, BuiltIn: row.EventType == ledger.DefaultEventType.Name},
		Metadata:             row.Metadata,
		IdempotencyKey:       row.IdempotencyKey,
		PendingTransferID:    row.PendingTransferID,
		ResolutionTransferID: row.ResolutionTransferID,
		TBLedger:             row.Source.TBLedger,
	}
	if row.TimeoutSeconds > 0 {
		expires := v.CreatedAt.Add(time.Duration(row.TimeoutSeconds) * time.Second)
		v.ExpiresAt = &expires
	}
	return v
}

// views reads hold and capture amounts from TigerBeetle in one batch.
func (u *Holds) views(ctx context.Context, rows []HoldRow) ([]HoldView, error) {
	ids := make([]ledger.ID, 0, 2*len(rows))
	for i := range rows {
		r := &rows[i]
		ids = append(ids, r.PendingTransferID)
		if r.ResolutionTransferID != nil {
			ids = append(ids, *r.ResolutionTransferID)
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
	out := make([]HoldView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		amount, ok := amounts[r.PendingTransferID]
		if !ok {
			return nil, fmt.Errorf("%w: pending transfer of hold %s is missing in TigerBeetle", ledger.ErrUnavailable, r.ID)
		}
		var captured *ledger.Amount
		if r.Status == ledger.HoldCaptured && r.ResolutionTransferID != nil {
			if c, ok := amounts[*r.ResolutionTransferID]; ok {
				captured = &c
			}
		}
		out = append(out, u.view(*r, amount, captured))
	}
	return out, nil
}

// Get returns a hold.
func (u *Holds) Get(ctx context.Context, ledgerName, id string) (HoldView, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return HoldView{}, err
	}
	holdID, err := ledger.ParseID(id)
	if err != nil {
		return HoldView{}, err
	}
	row, err := u.holds.GetHold(ctx, ledgerName, holdID)
	if err != nil {
		return HoldView{}, fmt.Errorf("get hold %s: %w", holdID, err)
	}
	views, err := u.views(ctx, []HoldRow{row})
	if err != nil {
		return HoldView{}, err
	}
	return views[0], nil
}

// List returns holds newest first with the count per status.
func (u *Holds) List(ctx context.Context, ledgerName, status, address, cursor string, limit int) ([]HoldView, string, HoldCounts, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return nil, "", HoldCounts{}, err
	}
	filter := HoldFilter{Status: ledger.HoldStatus(status)}
	switch filter.Status {
	case "", ledger.HoldOpen, ledger.HoldCaptured, ledger.HoldVoided, ledger.HoldExpired:
	default:
		return nil, "", HoldCounts{}, fmt.Errorf("%w: status must be open, captured, voided or expired", ledger.ErrInvalid)
	}
	p, err := parsePrefix(address)
	if err != nil {
		return nil, "", HoldCounts{}, err
	}
	filter.Address = p
	before, err := decodeIDCursor(cursor)
	if err != nil {
		return nil, "", HoldCounts{}, err
	}
	limit = pageSize(limit)
	now := u.clock.Now()
	rows, err := u.holds.ListHolds(ctx, ledgerName, filter, before, limit+1, now)
	if err != nil {
		return nil, "", HoldCounts{}, fmt.Errorf("list holds: %w", err)
	}
	page, next := trimPage(rows, limit, func(r HoldRow) string { return r.ID.String() })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", HoldCounts{}, err
	}
	counts, err := u.holds.CountHolds(ctx, ledgerName, now)
	if err != nil {
		return nil, "", HoldCounts{}, fmt.Errorf("count holds: %w", err)
	}
	return views, next, counts, nil
}

// settlePending is the reconciler path for hold, capture and void records.
func (u *Holds) settlePending(ctx context.Context, rec WriteRecord) (bool, error) {
	found, err := u.tb.LookupTransfers(ctx, rec.TransferIDs)
	if err != nil {
		return false, fmt.Errorf("lookup transfers: %w", err)
	}
	landed := len(found) == len(rec.TransferIDs)
	if rec.Operation == ledger.OperationHold {
		var req holdRequest
		if err := json.Unmarshal(rec.Request, &req); err != nil {
			return false, fmt.Errorf("decode stored request of %s: %w", rec.Key, err)
		}
		if !landed {
			_, err := u.executeCreate(ctx, rec, req)
			return settledBy(err)
		}
		posting, err := parseHoldPosting(req)
		if err != nil {
			return false, err
		}
		event, err := resolveEventType(ctx, u.events, rec.Ledger, req.EventType)
		if err != nil {
			return false, err
		}
		src, dst, err := u.holdAccounts(ctx, rec.Ledger, posting)
		if err != nil {
			return false, err
		}
		_, err = u.settleCreate(ctx, rec, req, event, src, dst, posting, chainVerdict{verdict: verdictCommitted, timestamp: found[0].Timestamp, failed: -1})
		return err == nil, err
	}
	var req resolutionRequest
	if err := json.Unmarshal(rec.Request, &req); err != nil {
		return false, fmt.Errorf("decode stored request of %s: %w", rec.Key, err)
	}
	hold, err := u.holds.GetHold(ctx, rec.Ledger, rec.TargetID)
	if err != nil {
		return false, fmt.Errorf("get hold %s: %w", rec.TargetID, err)
	}
	if !landed {
		_, err := u.executeResolve(ctx, rec, req, hold)
		return settledBy(err)
	}
	_, err = u.settleResolve(ctx, rec, hold, chainVerdict{verdict: verdictCommitted, timestamp: found[0].Timestamp, failed: -1})
	return err == nil, err
}

// settledBy reports whether an execution resolved the record: success, or a recorded decline.
func settledBy(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	var p *ledger.Problem
	if errors.As(err, &p) && !errors.Is(p, ledger.ErrUnavailable) {
		return true, nil
	}
	return false, err
}
