// Package tigerbeetle implements the bookkeeping module's ledger store and ID generator on the
// client of internal/platform/tigerbeetle. It owns the mapping between TigerBeetle types and the
// domain: identifier byte order, amounts, flags and result names.
package tigerbeetle

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	tb "github.com/tigerbeetle/tigerbeetle-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	platformtb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/tigerbeetle"
)

// Store implements domain.LedgerStore and domain.IDGenerator on the process's TigerBeetle client.
type Store struct {
	client *platformtb.Client
}

// NewStore builds the store on the client.
func NewStore(client *platformtb.Client) *Store {
	return &Store{client: client}
}

// Ready performs a round trip to the cluster.
func (s *Store) Ready(ctx context.Context) error {
	return s.client.Ready(ctx)
}

// NewID mints a time-ordered 128-bit identifier with the client's helper (ADR-0006).
func (*Store) NewID() ledger.ID {
	return ToDomainID(tb.ID())
}

// amountOf converts a client Uint128 (little-endian) to a domain amount.
func amountOf(v tb.Uint128) ledger.Amount {
	b := v.Bytes()
	var lo, hi uint64
	for i := 7; i >= 0; i-- {
		lo = lo<<8 | uint64(b[i])
		hi = hi<<8 | uint64(b[i+8])
	}
	return ledger.AmountFromParts(hi, lo)
}

// uint128Of converts a domain amount to the client representation.
func uint128Of(a ledger.Amount) tb.Uint128 {
	hi, lo := a.Parts()
	var b [16]byte
	for i := range 8 {
		b[i] = byte(lo >> (8 * i))
		b[i+8] = byte(hi >> (8 * i))
	}
	return tb.BytesToUint128(b)
}

func accountFlags(spec domain.AccountSpec) uint16 {
	f := tb.AccountFlags{History: spec.History}
	switch spec.Limit {
	case ledger.LimitDebitsMustNotExceedCredits:
		f.DebitsMustNotExceedCredits = true
	case ledger.LimitCreditsMustNotExceedDebits:
		f.CreditsMustNotExceedDebits = true
	case ledger.LimitNone:
	}
	return f.ToUint16()
}

// CreateAccounts creates the accounts. exists counts as created: the fields come from the stored
// row, so a repeat is identical (ADR-0006). Any other result is an invariant violation.
func (s *Store) CreateAccounts(ctx context.Context, specs []domain.AccountSpec) error {
	if len(specs) == 0 {
		return nil
	}
	accounts := make([]tb.Account, len(specs))
	for i, s := range specs {
		accounts[i] = tb.Account{
			ID:     FromDomainID(s.ID),
			Ledger: s.TBLedger,
			Code:   s.Code,
			Flags:  accountFlags(s),
		}
	}
	results, err := platformtb.Call(ctx, s.client, "create_accounts", func(client tb.Client) ([]tb.CreateAccountResult, error) { return client.CreateAccounts(accounts) })
	if err != nil {
		return err
	}
	for i, r := range results {
		if r.Status == tb.AccountCreated || r.Status == tb.AccountExists {
			continue
		}
		return fmt.Errorf("tigerbeetle: create_accounts: account %s: %s; the account row and TigerBeetle disagree, which is a bug", ToDomainID(accounts[i].ID), resultName(r.Status.String(), "Account"))
	}
	return nil
}

func transferFlags(s domain.TransferSpec) uint16 {
	f := tb.TransferFlags{Linked: s.Linked}
	switch s.Kind {
	case domain.TransferPending:
		f.Pending = true
	case domain.TransferPostPending:
		f.PostPendingTransfer = true
	case domain.TransferVoidPending:
		f.VoidPendingTransfer = true
	case domain.TransferPosting:
	}
	return f.ToUint16()
}

// CreateTransfers submits the transfers in one request and returns one outcome per transfer.
func (s *Store) CreateTransfers(ctx context.Context, specs []domain.TransferSpec) ([]domain.TransferOutcome, error) {
	transfers := make([]tb.Transfer, len(specs))
	for i := range specs {
		s := &specs[i]
		t := tb.Transfer{
			ID:              FromDomainID(s.ID),
			DebitAccountID:  FromDomainID(s.DebitAccountID),
			CreditAccountID: FromDomainID(s.CreditAccountID),
			Amount:          uint128Of(s.Amount),
			UserData128:     FromDomainID(s.UserData128),
			UserData64:      s.UserData64,
			Ledger:          s.TBLedger,
			Code:            s.Code,
			Flags:           transferFlags(*s),
			Timeout:         s.TimeoutSeconds,
		}
		if !s.PendingID.IsZero() {
			t.PendingID = FromDomainID(s.PendingID)
		}
		transfers[i] = t
	}
	results, err := platformtb.Call(ctx, s.client, "create_transfers", func(client tb.Client) ([]tb.CreateTransferResult, error) { return client.CreateTransfers(transfers) })
	if err != nil {
		return nil, err
	}
	out := make([]domain.TransferOutcome, len(results))
	for i, r := range results {
		out[i] = domain.TransferOutcome{Result: resultName(r.Status.String(), "Transfer"), Timestamp: r.Timestamp}
	}
	return out, nil
}

// LookupAccounts returns the accounts that exist.
func (s *Store) LookupAccounts(ctx context.Context, ids []ledger.ID) ([]domain.AccountState, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tbIDs := make([]tb.Uint128, len(ids))
	for i, id := range ids {
		tbIDs[i] = FromDomainID(id)
	}
	accounts, err := platformtb.Call(ctx, s.client, "lookup_accounts", func(client tb.Client) ([]tb.Account, error) { return client.LookupAccounts(tbIDs) })
	if err != nil {
		return nil, err
	}
	out := make([]domain.AccountState, len(accounts))
	for i := range accounts {
		a := &accounts[i]
		out[i] = domain.AccountState{
			ID:             ToDomainID(a.ID),
			DebitsPending:  amountOf(a.DebitsPending),
			DebitsPosted:   amountOf(a.DebitsPosted),
			CreditsPending: amountOf(a.CreditsPending),
			CreditsPosted:  amountOf(a.CreditsPosted),
			Timestamp:      a.Timestamp,
		}
	}
	return out, nil
}

// LookupTransfers returns the transfers that exist.
func (s *Store) LookupTransfers(ctx context.Context, ids []ledger.ID) ([]domain.TransferRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tbIDs := make([]tb.Uint128, len(ids))
	for i, id := range ids {
		tbIDs[i] = FromDomainID(id)
	}
	transfers, err := platformtb.Call(ctx, s.client, "lookup_transfers", func(client tb.Client) ([]tb.Transfer, error) { return client.LookupTransfers(tbIDs) })
	if err != nil {
		return nil, err
	}
	out := make([]domain.TransferRecord, len(transfers))
	for i := range transfers {
		t := &transfers[i]
		f := t.TransferFlags()
		out[i] = domain.TransferRecord{
			ID:              ToDomainID(t.ID),
			DebitAccountID:  ToDomainID(t.DebitAccountID),
			CreditAccountID: ToDomainID(t.CreditAccountID),
			Amount:          amountOf(t.Amount),
			PendingID:       ToDomainID(t.PendingID),
			UserData128:     ToDomainID(t.UserData128),
			UserData64:      t.UserData64,
			TBLedger:        t.Ledger,
			Code:            t.Code,
			Linked:          f.Linked,
			Pending:         f.Pending,
			PostPending:     f.PostPendingTransfer,
			VoidPending:     f.VoidPendingTransfer,
			TimeoutSeconds:  t.Timeout,
			Timestamp:       t.Timestamp,
		}
	}
	return out, nil
}

// AccountBalances returns historical balances, newest first. Only accounts created with the
// history flag have them.
func (s *Store) AccountBalances(ctx context.Context, id ledger.ID, limit int) ([]domain.BalancePoint, error) {
	filter := tb.AccountFilter{
		AccountID: FromDomainID(id),
		Limit:     uint32(limit),
		Flags:     tb.AccountFilterFlags{Debits: true, Credits: true, Reversed: true}.ToUint32(),
	}
	balances, err := platformtb.Call(ctx, s.client, "get_account_balances", func(client tb.Client) ([]tb.AccountBalance, error) { return client.GetAccountBalances(filter) })
	if err != nil {
		return nil, err
	}
	out := make([]domain.BalancePoint, len(balances))
	for i := range balances {
		b := &balances[i]
		out[i] = domain.BalancePoint{
			DebitsPending:  amountOf(b.DebitsPending),
			DebitsPosted:   amountOf(b.DebitsPosted),
			CreditsPending: amountOf(b.CreditsPending),
			CreditsPosted:  amountOf(b.CreditsPosted),
			Timestamp:      b.Timestamp,
		}
	}
	return out, nil
}

// resultName turns the client's status name into the name TigerBeetle documents:
// "TransferExceedsCredits" becomes "exceeds_credits", "TransferIDAlreadyFailed" becomes
// "id_already_failed". An underscore starts every word, and an acronym is one word.
func resultName(status, prefix string) string {
	runes := []rune(strings.TrimPrefix(status, prefix))
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prevLower := unicode.IsLower(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevLower || (unicode.IsUpper(runes[i-1]) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
