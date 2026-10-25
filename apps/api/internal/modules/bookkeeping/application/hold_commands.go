package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

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

type holdCommands struct {
	ledgers  LedgerDirectory
	events   EventTypes
	holds    domain.HoldRepository
	accounts *accountEnsurer
	flow     *writeFlow
	tb       domain.LedgerStore
	ids      domain.IDGenerator
	clock    ledger.Clock
	queries  *holdQueries
}

// Create reserves the amount with a pending transfer. The source's available balance drops at
// once, because debits_must_not_exceed_credits counts pending debits.
func (u *holdCommands) Create(ctx context.Context, in CreateHoldInput) (HoldView, error) {
	if err := domain.ValidateIdempotencyKey(in.Key); err != nil {
		return HoldView{}, err
	}
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return HoldView{}, err
	}
	if err := in.Posting.Validate(0); err != nil {
		return HoldView{}, err
	}
	if err := domain.ValidateReference(in.Reference); err != nil {
		return HoldView{}, err
	}
	if in.Metadata == nil {
		in.Metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(in.Metadata); err != nil {
		return HoldView{}, err
	}
	event, err := u.events.Resolve(ctx, in.Ledger, in.EventType)
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
	rec := domain.WriteRecord{
		Ledger:      in.Ledger,
		Key:         in.Key,
		Operation:   domain.OperationHold,
		Status:      domain.RequestPending,
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
		return u.queries.Get(ctx, in.Ledger, stored.TargetID.String())
	}
	return u.executeCreate(ctx, stored, req)
}

func (u *holdCommands) executeCreate(ctx context.Context, rec domain.WriteRecord, req holdRequest) (HoldView, error) {
	posting, err := parsePosting(req.Source, req.Destination, req.Asset, req.Amount)
	if err != nil {
		return HoldView{}, u.flow.decline(ctx, rec, domain.RequestFailed, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", err.Error()))
	}
	event, err := u.events.Resolve(ctx, rec.Ledger, req.EventType)
	if err != nil {
		var p *ledger.Problem
		if errors.As(err, &p) {
			return HoldView{}, u.flow.decline(ctx, rec, domain.RequestFailed, p)
		}
		return HoldView{}, err
	}
	src, dst, err := u.holdAccounts(ctx, rec.Ledger, posting)
	if err != nil {
		return HoldView{}, u.flow.pending(ctx, rec, err)
	}
	transfer := domain.TransferSpec{
		ID:              rec.TransferIDs[0],
		DebitAccountID:  src.ID,
		CreditAccountID: dst.ID,
		Amount:          posting.Amount,
		TBLedger:        src.TBLedger,
		Code:            event.Code,
		Kind:            domain.TransferPending,
		TimeoutSeconds:  req.TimeoutSeconds,
		UserData128:     rec.TargetID,
	}
	outcomes, err := u.flow.submit(ctx, rec, []domain.TransferSpec{transfer})
	if err != nil {
		return HoldView{}, err
	}
	return u.settleCreate(ctx, rec, req, event, src, dst, posting, domain.Classify(outcomes))
}

func (u *holdCommands) holdAccounts(ctx context.Context, ledgerName string, posting domain.Posting) (src, dst domain.AccountRow, err error) {
	srcKey := domain.AccountKey{Address: posting.Source, Asset: posting.Asset.String()}
	dstKey := domain.AccountKey{Address: posting.Destination, Asset: posting.Asset.String()}
	accounts, err := u.accounts.ensure(ctx, ledgerName, []domain.AccountKey{srcKey, dstKey})
	if err != nil {
		return domain.AccountRow{}, domain.AccountRow{}, err
	}
	return accounts[srcKey], accounts[dstKey], nil
}

func (u *holdCommands) settleCreate(ctx context.Context, rec domain.WriteRecord, req holdRequest, event ledger.EventType,
	src, dst domain.AccountRow, posting domain.Posting, v domain.ChainVerdict,
) (HoldView, error) {
	switch v.Verdict {
	case domain.VerdictCommitted:
		ts := v.Timestamp
		if ts == 0 {
			var err error
			if ts, err = latestTimestamp(ctx, u.tb, rec.TransferIDs); err != nil {
				return HoldView{}, err
			}
		}
		row := domain.HoldRow{
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
			CreatedAt:         domain.TimeFromTB(ts),
			TBTimestamp:       ts,
			Status:            domain.HoldOpen,
		}
		if err := u.flow.writes.CommitHold(ctx, u.flow.committedOutcome(rec, domain.StatusCreated, rec.TargetID), row); err != nil {
			return HoldView{}, fmt.Errorf("commit hold: %w", err)
		}
		return u.queries.view(row, posting.Amount, nil), nil
	case domain.VerdictBurned:
		return HoldView{}, u.flow.decline(ctx, rec, domain.RequestFailed, &ledger.Problem{
			Kind: ledger.ErrRejected, Reason: "hold-declined", Posting: -1, Results: v.Results,
			Detail: "TigerBeetle declined this hold on an earlier attempt and no longer reports why; nothing was reserved. Retry with a new Idempotency-Key",
		})
	default:
		p := &ledger.Problem{Kind: ledger.ErrRejected, Reason: domain.ResultReason(v.Result), Posting: -1, Results: v.Results}
		switch v.Result {
		case domain.ResultExceedsCredits:
			p.Detail = fmt.Sprintf("The hold would take %s below zero in %s: its available balance, which already counts open holds, is lower than the amount.", posting.Source, posting.Asset)
		case domain.ResultExceedsDebits:
			p.Detail = fmt.Sprintf("The hold would take %s above its limit in %s (credits must not exceed debits).", posting.Destination, posting.Asset)
		default:
			p.Detail = fmt.Sprintf("TigerBeetle declined the hold with %s; nothing was reserved.", v.Result)
		}
		return HoldView{}, u.flow.decline(ctx, rec, domain.RequestDeclined, p)
	}
}

func (u *holdCommands) Capture(ctx context.Context, in CaptureHoldInput) (HoldView, error) {
	req := resolutionRequest{Hold: in.ID}
	if in.Amount != nil {
		if in.Amount.IsZero() {
			// TigerBeetle would read a zero post as "void the rest", turning a capture into a void.
			return HoldView{}, fmt.Errorf("%w: capture amount must be greater than zero; void the hold to release it", ledger.ErrInvalid)
		}
		req.Amount = in.Amount.String()
	}
	return u.resolve(ctx, in.Ledger, in.Key, in.RequestID, domain.OperationCapture, req)
}

func (u *holdCommands) Void(ctx context.Context, in VoidHoldInput) (HoldView, error) {
	return u.resolve(ctx, in.Ledger, in.Key, in.RequestID, domain.OperationVoid, resolutionRequest{Hold: in.ID})
}

func (u *holdCommands) resolve(ctx context.Context, ledgerName, key, requestID string, op domain.Operation, req resolutionRequest) (HoldView, error) {
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		return HoldView{}, err
	}
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
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
	rec := domain.WriteRecord{
		Ledger:      ledgerName,
		Key:         key,
		Operation:   op,
		Status:      domain.RequestPending,
		BodyHash:    hash,
		Request:     body,
		TargetID:    holdID,
		TransferIDs: []ledger.ID{u.ids.NewID()},
		RequestID:   requestID,
		CreatedAt:   u.clock.Now().UTC(),
	}
	if hold.Status == domain.HoldCaptured || hold.Status == domain.HoldVoided {
		// A replay of the write that resolved the hold answers with the hold; any other write is
		// refused before minting anything.
		if existing, err := u.flow.writes.GetWrite(ctx, ledgerName, key); err == nil && existing.Operation == op && bytes.Equal(existing.BodyHash, hash) {
			return u.queries.Get(ctx, ledgerName, holdID.String())
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
		return u.queries.Get(ctx, ledgerName, holdID.String())
	}
	return u.executeResolve(ctx, stored, req, hold)
}

func alreadyResolved(hold domain.HoldRow) error {
	return ledger.NewProblem(ledger.ErrConflict, "hold-already-resolved", fmt.Sprintf("hold %s is already %s; a hold is captured or voided exactly once", hold.ID, hold.Status))
}

func (u *holdCommands) executeResolve(ctx context.Context, rec domain.WriteRecord, req resolutionRequest, hold domain.HoldRow) (HoldView, error) {
	transfer := domain.TransferSpec{
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
	if rec.Operation == domain.OperationCapture {
		transfer.Kind = domain.TransferPostPending
		transfer.Amount = ledger.MaxAmount
		if req.Amount != "" {
			amount, err := ledger.ParseAmount(req.Amount)
			if err != nil {
				return HoldView{}, err
			}
			transfer.Amount = amount
		}
	} else {
		transfer.Kind = domain.TransferVoidPending
	}
	outcomes, err := u.flow.submit(ctx, rec, []domain.TransferSpec{transfer})
	if err != nil {
		return HoldView{}, err
	}
	return u.settleResolve(ctx, rec, hold, domain.Classify(outcomes))
}

func (u *holdCommands) settleResolve(ctx context.Context, rec domain.WriteRecord, hold domain.HoldRow, v domain.ChainVerdict) (HoldView, error) {
	status := domain.HoldCaptured
	if rec.Operation == domain.OperationVoid {
		status = domain.HoldVoided
	}
	switch v.Verdict {
	case domain.VerdictCommitted:
		ts := v.Timestamp
		if ts == 0 {
			var err error
			if ts, err = latestTimestamp(ctx, u.tb, rec.TransferIDs); err != nil {
				return HoldView{}, err
			}
		}
		resolution := domain.HoldResolution{Ledger: rec.Ledger, HoldID: hold.ID, Status: status, ResolutionTransferID: rec.TransferIDs[0], ResolvedAt: domain.TimeFromTB(ts)}
		if err := u.flow.writes.CommitResolution(ctx, u.flow.committedOutcome(rec, domain.StatusOK, hold.ID), resolution); err != nil {
			return HoldView{}, fmt.Errorf("commit %s: %w", rec.Operation, err)
		}
		return u.queries.Get(ctx, rec.Ledger, hold.ID.String())
	case domain.VerdictBurned:
		return HoldView{}, u.flow.decline(ctx, rec, domain.RequestFailed, &ledger.Problem{
			Kind: ledger.ErrRejected, Reason: "hold-resolution-declined", Posting: -1, Results: v.Results,
			Detail: "TigerBeetle declined this write on an earlier attempt and no longer reports why. Retry with a new Idempotency-Key",
		})
	case domain.VerdictDeclined:
		// Explained below, per TigerBeetle result.
	}
	p := &ledger.Problem{Kind: ledger.ErrRejected, Reason: domain.ResultReason(v.Result), Posting: -1, Results: v.Results}
	switch v.Result {
	case domain.ResultPendingTransferExpired:
		if err := u.holds.MarkHoldExpired(ctx, rec.Ledger, hold.ID); err != nil {
			return HoldView{}, fmt.Errorf("mark hold expired: %w", err)
		}
		p.Kind, p.Reason = ledger.ErrConflict, "hold-expired"
		p.Detail = fmt.Sprintf("hold %s expired on the cluster clock and TigerBeetle released it; it can no longer be captured or voided", hold.ID)
	case domain.ResultPendingTransferAlreadyPosted, domain.ResultPendingTransferAlreadyVoided:
		p.Kind, p.Reason = ledger.ErrConflict, "hold-already-resolved"
		p.Detail = fmt.Sprintf("hold %s was already resolved; a hold is captured or voided exactly once", hold.ID)
	case domain.ResultExceedsPendingTransferAmount:
		p.Kind, p.Reason = ledger.ErrInvalid, "capture-exceeds-hold"
		p.Detail = "the capture amount is larger than the hold"
	default:
		p.Detail = fmt.Sprintf("TigerBeetle declined the %s with %s", rec.Operation, v.Result)
	}
	return HoldView{}, u.flow.decline(ctx, rec, domain.RequestDeclined, p)
}

// settlePending is the reconciler path for hold, capture and void records.
func (u *holdCommands) settlePending(ctx context.Context, rec domain.WriteRecord) (bool, error) {
	found, err := u.tb.LookupTransfers(ctx, rec.TransferIDs)
	if err != nil {
		return false, fmt.Errorf("lookup transfers: %w", err)
	}
	landed := len(found) == len(rec.TransferIDs)
	if rec.Operation == domain.OperationHold {
		var req holdRequest
		if err := json.Unmarshal(rec.Request, &req); err != nil {
			return false, fmt.Errorf("decode stored request of %s: %w", rec.Key, err)
		}
		if !landed {
			_, err := u.executeCreate(ctx, rec, req)
			return settledBy(err)
		}
		posting, err := parsePosting(req.Source, req.Destination, req.Asset, req.Amount)
		if err != nil {
			return false, err
		}
		event, err := u.events.Resolve(ctx, rec.Ledger, req.EventType)
		if err != nil {
			return false, err
		}
		src, dst, err := u.holdAccounts(ctx, rec.Ledger, posting)
		if err != nil {
			return false, err
		}
		_, err = u.settleCreate(ctx, rec, req, event, src, dst, posting, domain.ChainVerdict{Verdict: domain.VerdictCommitted, Timestamp: found[0].Timestamp, Failed: -1})
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
	_, err = u.settleResolve(ctx, rec, hold, domain.ChainVerdict{Verdict: domain.VerdictCommitted, Timestamp: found[0].Timestamp, Failed: -1})
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
