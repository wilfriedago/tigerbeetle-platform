package v1

import (
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// The converters below read the dto wire types REST and GraphQL answer with, so every transport
// shows the same values; only the encoding differs.

func balanceOf(b *dto.AssetBalance) *pb.AssetBalance {
	return &pb.AssetBalance{
		Asset:     string(b.Asset),
		AccountId: string(b.AccountID),
		TbLedger:  b.TbLedger,
		Balance:   string(b.Balance),
		Available: string(b.Available),
		Reserved:  string(b.Reserved),
		Incoming:  string(b.Incoming),
		Credits:   string(b.Credits),
		Debits:    string(b.Debits),
		Limit:     grpcapi.EnumOf[pb.BalanceLimit](string(b.Limit)),
		History:   b.History,
		Code:      uint32(b.Code),
		Rule:      b.Rule,
		CreatedAt: grpcapi.Timestamp(b.CreatedAt),
	}
}

func balancesOf(in []dto.AssetBalance) []*pb.AssetBalance {
	out := make([]*pb.AssetBalance, len(in))
	for i := range in {
		out[i] = balanceOf(&in[i])
	}
	return out
}

func accountSummaryOf(a *dto.AccountSummary) *pb.AccountSummary {
	return &pb.AccountSummary{
		Address:   string(a.Address),
		CreatedAt: grpcapi.Timestamp(a.CreatedAt),
		Metadata:  a.Metadata.Map(),
		Balances:  balancesOf(a.Balances),
	}
}

func accountOf(a *dto.Account) *pb.Account {
	return &pb.Account{
		Address:   string(a.Address),
		CreatedAt: grpcapi.Timestamp(a.CreatedAt),
		Metadata:  a.Metadata.Map(),
		Balances:  balancesOf(a.Balances),
		OpenHolds: a.OpenHolds,
	}
}

func eventTypeRefOf(e dto.EventTypeRef) *pb.EventTypeRef {
	return &pb.EventTypeRef{Name: e.Name, Code: uint32(e.Code)}
}

func transactionOf(t *dto.Transaction) *pb.Transaction {
	postings := make([]*pb.PostingRecord, len(t.Postings))
	for i := range t.Postings {
		p := &t.Postings[i]
		postings[i] = &pb.PostingRecord{
			Source: string(p.Source), Destination: string(p.Destination), Asset: string(p.Asset),
			Amount: string(p.Amount), TransferId: string(p.TransferID), TbLedger: p.TbLedger,
		}
	}
	return &pb.Transaction{
		Id:             string(t.ID),
		Ledger:         string(t.Ledger),
		Postings:       postings,
		Reference:      t.Reference,
		EventType:      eventTypeRefOf(t.EventType),
		BusinessTime:   grpcapi.OptionalTimestamp(t.BusinessTime),
		Metadata:       t.Metadata.Map(),
		CommittedAt:    grpcapi.Timestamp(t.CommittedAt),
		Timestamp:      string(t.Timestamp),
		Reverts:        optionalID(t.Reverts),
		RevertedBy:     optionalID(t.RevertedBy),
		IdempotencyKey: t.IdempotencyKey,
		RequestId:      t.RequestID,
	}
}

func holdOf(h *dto.Hold) *pb.Hold {
	out := &pb.Hold{
		Id:                   string(h.ID),
		Ledger:               string(h.Ledger),
		Source:               string(h.Source),
		Destination:          string(h.Destination),
		Asset:                string(h.Asset),
		Amount:               string(h.Amount),
		Status:               grpcapi.EnumOf[pb.HoldStatus](string(h.Status)),
		TimeoutSeconds:       h.TimeoutSeconds,
		CreatedAt:            grpcapi.Timestamp(h.CreatedAt),
		Timestamp:            string(h.Timestamp),
		ExpiresAt:            grpcapi.OptionalTimestamp(h.ExpiresAt),
		ResolvedAt:           grpcapi.OptionalTimestamp(h.ResolvedAt),
		Reference:            h.Reference,
		EventType:            eventTypeRefOf(h.EventType),
		Metadata:             h.Metadata.Map(),
		IdempotencyKey:       h.IdempotencyKey,
		PendingTransferId:    string(h.PendingTransferID),
		ResolutionTransferId: optionalID(h.ResolutionTransferID),
		TbLedger:             h.TbLedger,
	}
	if h.CapturedAmount != nil {
		out.CapturedAmount = grpcapi.Optional(*h.CapturedAmount)
	}
	return out
}

func holdStatsOf(s *dto.HoldStats) *pb.HoldStats {
	reserved := make([]*pb.AssetAmount, len(s.Reserved))
	for i, r := range s.Reserved {
		reserved[i] = &pb.AssetAmount{Asset: string(r.Asset), Amount: string(r.Amount)}
	}
	return &pb.HoldStats{Open: s.Open, Captured: s.Captured, Voided: s.Voided, Expired: s.Expired, Reserved: reserved}
}

func writeRequestOf(r *dto.WriteRequest) *pb.WriteRequest {
	out := &pb.WriteRequest{
		Key:         r.Key,
		Operation:   grpcapi.EnumOf[pb.Operation](string(r.Operation)),
		Status:      grpcapi.EnumOf[pb.RequestStatus](string(r.Status)),
		CreatedAt:   grpcapi.Timestamp(r.CreatedAt),
		ResolvedAt:  grpcapi.OptionalTimestamp(r.ResolvedAt),
		TargetId:    string(r.TargetID),
		ProblemType: r.ProblemType,
		RequestId:   r.RequestID,
		BodyHash:    r.BodyHash,
		Attempts:    int32(r.Attempts),
	}
	if r.ResponseStatus != nil {
		status := int32(*r.ResponseStatus)
		out.ResponseStatus = &status
	}
	return out
}

func assetsOf(in []scalar.Asset) []string {
	out := make([]string, len(in))
	for i, a := range in {
		out[i] = string(a)
	}
	return out
}

func optionalID(id *scalar.ID) *string {
	if id == nil {
		return nil
	}
	return grpcapi.Optional(*id)
}
