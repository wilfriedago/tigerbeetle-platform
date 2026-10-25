package tigerbeetle

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	tb "github.com/tigerbeetle/tigerbeetle-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// call runs a client request and waits for it or for the context. The client has no per-call
// cancellation and retries an unreachable cluster indefinitely, so on a deadline the request keeps
// running in its goroutine and may still land. Writes cope with that through the pending
// idempotency record and the reconciler (ADR-0007).
func call[T any](ctx context.Context, timeout time.Duration, op string, fn func() (T, error)) (T, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	type result struct {
		value T
		err   error
	}
	done := make(chan result, 1)
	go func() {
		v, err := fn()
		done <- result{value: v, err: err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			var zero T
			return zero, fmt.Errorf("tigerbeetle: %s: %w", op, mapClientError(r.err))
		}
		return r.value, nil
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("tigerbeetle: %s: %w: %w", op, ledger.ErrUnavailable, ctx.Err())
	}
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

func accountFlags(spec app.AccountSpec) uint16 {
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
func (c *Client) CreateAccounts(ctx context.Context, specs []app.AccountSpec) error {
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
	results, err := call(ctx, c.timeout, "create_accounts", func() ([]tb.CreateAccountResult, error) { return c.tb.CreateAccounts(accounts) })
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

func transferFlags(s app.TransferSpec) uint16 {
	f := tb.TransferFlags{Linked: s.Linked}
	switch s.Kind {
	case app.TransferPending:
		f.Pending = true
	case app.TransferPostPending:
		f.PostPendingTransfer = true
	case app.TransferVoidPending:
		f.VoidPendingTransfer = true
	case app.TransferPosting:
	}
	return f.ToUint16()
}

// CreateTransfers submits the transfers in one request and returns one outcome per transfer.
func (c *Client) CreateTransfers(ctx context.Context, specs []app.TransferSpec) ([]app.TransferOutcome, error) {
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
	results, err := call(ctx, c.timeout, "create_transfers", func() ([]tb.CreateTransferResult, error) { return c.tb.CreateTransfers(transfers) })
	if err != nil {
		return nil, err
	}
	out := make([]app.TransferOutcome, len(results))
	for i, r := range results {
		out[i] = app.TransferOutcome{Result: resultName(r.Status.String(), "Transfer"), Timestamp: r.Timestamp}
	}
	return out, nil
}

// LookupAccounts returns the accounts that exist.
func (c *Client) LookupAccounts(ctx context.Context, ids []ledger.ID) ([]app.AccountState, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tbIDs := make([]tb.Uint128, len(ids))
	for i, id := range ids {
		tbIDs[i] = FromDomainID(id)
	}
	accounts, err := call(ctx, c.timeout, "lookup_accounts", func() ([]tb.Account, error) { return c.tb.LookupAccounts(tbIDs) })
	if err != nil {
		return nil, err
	}
	out := make([]app.AccountState, len(accounts))
	for i := range accounts {
		a := &accounts[i]
		out[i] = app.AccountState{
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
func (c *Client) LookupTransfers(ctx context.Context, ids []ledger.ID) ([]app.TransferRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tbIDs := make([]tb.Uint128, len(ids))
	for i, id := range ids {
		tbIDs[i] = FromDomainID(id)
	}
	transfers, err := call(ctx, c.timeout, "lookup_transfers", func() ([]tb.Transfer, error) { return c.tb.LookupTransfers(tbIDs) })
	if err != nil {
		return nil, err
	}
	out := make([]app.TransferRecord, len(transfers))
	for i := range transfers {
		t := &transfers[i]
		f := t.TransferFlags()
		out[i] = app.TransferRecord{
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
func (c *Client) AccountBalances(ctx context.Context, id ledger.ID, limit int) ([]app.BalancePoint, error) {
	filter := tb.AccountFilter{
		AccountID: FromDomainID(id),
		Limit:     uint32(limit),
		Flags:     tb.AccountFilterFlags{Debits: true, Credits: true, Reversed: true}.ToUint32(),
	}
	balances, err := call(ctx, c.timeout, "get_account_balances", func() ([]tb.AccountBalance, error) { return c.tb.GetAccountBalances(filter) })
	if err != nil {
		return nil, err
	}
	out := make([]app.BalancePoint, len(balances))
	for i := range balances {
		b := &balances[i]
		out[i] = app.BalancePoint{
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
