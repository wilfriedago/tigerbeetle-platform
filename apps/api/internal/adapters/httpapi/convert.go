package httpapi

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Amounts and balances are decimal strings in JSON (ADR-0004); big.Int and ledger.Amount render
// exactly, never through a float.
func signed(v *big.Int) gen.SignedAmount {
	if v == nil {
		return "0"
	}
	return v.String()
}

func metadataOf(m map[string]string) gen.Metadata {
	if m == nil {
		return gen.Metadata{}
	}
	return m
}

func metadataIn(m *gen.Metadata) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return *m
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func optionalID(id *ledger.ID) *gen.Id {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func timestampOf(ts uint64) gen.TigerBeetleTimestamp {
	return strconv.FormatUint(ts, 10)
}

func assetsOf(assets []string) []gen.Asset {
	if assets == nil {
		return []gen.Asset{}
	}
	return assets
}

func ledgerOf(s app.LedgerSummary) gen.Ledger {
	return gen.Ledger{
		Name:      s.Name,
		Metadata:  metadataOf(s.Metadata),
		CreatedAt: s.CreatedAt,
		Assets:    assetsOf(s.Assets),
		Accounts:  s.Accounts,
	}
}

func balanceOf(b app.BalanceView) gen.AssetBalance {
	return gen.AssetBalance{
		Asset:     b.Asset,
		AccountId: b.ID.String(),
		TbLedger:  int64(b.TBLedger),
		Balance:   signed(b.Balance),
		Available: signed(b.Available),
		Reserved:  b.Reserved.String(),
		Incoming:  b.Incoming.String(),
		Credits:   b.Credits.String(),
		Debits:    b.Debits.String(),
		Limit:     gen.BalanceLimit(b.Limit),
		History:   b.History,
		Code:      int(b.Code),
		Rule:      b.RulePattern,
		CreatedAt: b.CreatedAt,
	}
}

func accountSummaryOf(v app.AccountView) gen.AccountSummary {
	balances := make([]gen.AssetBalance, len(v.Balances))
	for i := range v.Balances {
		b := &v.Balances[i]
		balances[i] = balanceOf(*b)
	}
	return gen.AccountSummary{Address: string(v.Address), CreatedAt: v.CreatedAt, Metadata: metadataOf(v.Metadata), Balances: balances}
}

func accountOf(v app.AccountView) gen.Account {
	s := accountSummaryOf(v)
	return gen.Account{Address: s.Address, CreatedAt: s.CreatedAt, Metadata: s.Metadata, Balances: s.Balances, OpenHolds: v.OpenHolds}
}

func eventTypeRefOf(e ledger.EventType) gen.EventTypeRef {
	return gen.EventTypeRef{Name: e.Name, Code: int(e.Code)}
}

func transactionOf(v app.TransactionView) gen.Transaction {
	postings := make([]gen.PostingRecord, len(v.Postings))
	for i, p := range v.Postings {
		postings[i] = gen.PostingRecord{
			Source:      string(p.Source),
			Destination: string(p.Destination),
			Asset:       p.Asset,
			Amount:      p.Amount.String(),
			TransferId:  p.TransferID.String(),
			TbLedger:    int64(p.TBLedger),
		}
	}
	return gen.Transaction{
		Id:             v.ID.String(),
		Ledger:         v.Ledger,
		Postings:       postings,
		Reference:      optional(v.Reference),
		EventType:      eventTypeRefOf(v.EventType),
		BusinessTime:   v.BusinessTime,
		Metadata:       metadataOf(v.Metadata),
		CommittedAt:    v.CommittedAt,
		Timestamp:      timestampOf(v.Timestamp),
		Reverts:        optionalID(v.Reverts),
		RevertedBy:     optionalID(v.RevertedBy),
		IdempotencyKey: v.IdempotencyKey,
		RequestId:      optional(v.RequestID),
	}
}

func holdOf(v app.HoldView) gen.Hold {
	h := gen.Hold{
		Id:                   v.ID.String(),
		Ledger:               v.Ledger,
		Source:               string(v.Source),
		Destination:          string(v.Destination),
		Asset:                v.Asset,
		Amount:               v.Amount.String(),
		Status:               gen.HoldStatus(v.Status),
		TimeoutSeconds:       int64(v.TimeoutSeconds),
		CreatedAt:            v.CreatedAt,
		Timestamp:            timestampOf(v.Timestamp),
		ExpiresAt:            v.ExpiresAt,
		ResolvedAt:           v.ResolvedAt,
		Reference:            optional(v.Reference),
		EventType:            eventTypeRefOf(v.EventType),
		Metadata:             metadataOf(v.Metadata),
		IdempotencyKey:       v.IdempotencyKey,
		PendingTransferId:    v.PendingTransferID.String(),
		ResolutionTransferId: optionalID(v.ResolutionTransferID),
	}
	tbLedger := int64(v.TBLedger)
	h.TbLedger = &tbLedger
	if v.CapturedAmount != nil {
		c := v.CapturedAmount.String()
		h.CapturedAmount = &c
	}
	return h
}

func holdStatsOf(c app.HoldCounts, reserved map[string]*big.Int) gen.HoldStats {
	s := gen.HoldStats{Open: c.Open, Captured: c.Captured, Voided: c.Voided, Expired: c.Expired, Reserved: []gen.AssetAmount{}}
	for asset, amount := range reserved {
		s.Reserved = append(s.Reserved, gen.AssetAmount{Asset: asset, Amount: amount.String()})
	}
	return s
}

func ruleOf(r ledger.Rule) gen.Rule {
	out := gen.Rule{
		Pattern:  string(r.Pattern),
		Limit:    gen.BalanceLimit(r.Limit),
		History:  r.History,
		Code:     int(r.Code),
		BuiltIn:  r.BuiltIn,
		Segments: r.Pattern.Segments(),
	}
	if !r.CreatedAt.IsZero() {
		created, updated := r.CreatedAt, r.UpdatedAt
		out.CreatedAt, out.UpdatedAt = &created, &updated
	}
	return out
}

func writeRequestOf(rec app.WriteRecord) gen.WriteRequest {
	w := gen.WriteRequest{
		Key:         rec.Key,
		Operation:   gen.Operation(rec.Operation),
		Status:      gen.RequestStatus(rec.Status),
		CreatedAt:   rec.CreatedAt,
		ResolvedAt:  rec.ResolvedAt,
		TargetId:    rec.TargetID.String(),
		ProblemType: problemTypeOf(rec.ProblemType),
		RequestId:   optional(rec.RequestID),
		BodyHash:    hex.EncodeToString(rec.BodyHash),
		Attempts:    rec.Attempts,
	}
	if rec.ResponseStatus != 0 {
		status := rec.ResponseStatus
		w.ResponseStatus = &status
	}
	return w
}

func problemTypeOf(reason string) *string {
	if reason == "" {
		return nil
	}
	t := "/problems/" + reason
	return &t
}
