package cdc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// The JSON of a TigerBeetle CDC message. TigerBeetle 0.17.9 writes u128 and u64 values as JSON
// numbers up to 2^53 - 1 and as strings above, and nanosecond timestamps always as strings
// (ADR-0022, spike S3), so those fields decode through u128 and u64, which accept both and never
// pass through a float.
type message struct {
	Type      string  `json:"type"`
	Timestamp u64     `json:"timestamp"`
	Ledger    uint32  `json:"ledger"`
	Transfer  xfer    `json:"transfer"`
	Debit     account `json:"debit_account"`
	Credit    account `json:"credit_account"`
}

type xfer struct {
	ID          u128   `json:"id"`
	Amount      u128   `json:"amount"`
	PendingID   u128   `json:"pending_id"`
	UserData128 u128   `json:"user_data_128"`
	UserData64  u64    `json:"user_data_64"`
	UserData32  uint32 `json:"user_data_32"`
	Timeout     uint32 `json:"timeout"`
	Code        uint16 `json:"code"`
	Flags       uint16 `json:"flags"`
	Timestamp   u64    `json:"timestamp"`
}

type account struct {
	ID             u128   `json:"id"`
	DebitsPending  u128   `json:"debits_pending"`
	DebitsPosted   u128   `json:"debits_posted"`
	CreditsPending u128   `json:"credits_pending"`
	CreditsPosted  u128   `json:"credits_posted"`
	UserData128    u128   `json:"user_data_128"`
	UserData64     u64    `json:"user_data_64"`
	UserData32     uint32 `json:"user_data_32"`
	Code           uint16 `json:"code"`
	Flags          uint16 `json:"flags"`
	Timestamp      u64    `json:"timestamp"`
}

// Decode reads one CDC message.
func Decode(body []byte) (domain.ChangeEvent, error) {
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return domain.ChangeEvent{}, fmt.Errorf("cdc: decode: %w", err)
	}
	switch m.Type {
	case domain.ChangeSinglePhase, domain.ChangeTwoPhasePending, domain.ChangeTwoPhasePosted,
		domain.ChangeTwoPhaseVoided, domain.ChangeTwoPhaseExpired:
	default:
		return domain.ChangeEvent{}, fmt.Errorf("cdc: unknown event type %q", m.Type)
	}
	t := m.Transfer
	return domain.ChangeEvent{
		Type: m.Type, Timestamp: uint64(m.Timestamp), TBLedger: m.Ledger,
		Transfer: domain.ChangeTransfer{
			ID: t.ID.id(), Amount: t.Amount.amount(), PendingID: t.PendingID.id(), UserData128: t.UserData128.id(),
			UserData64: uint64(t.UserData64), UserData32: t.UserData32, Timeout: t.Timeout, Code: t.Code, Flags: t.Flags,
			Timestamp: uint64(t.Timestamp),
		},
		Debit:  m.Debit.change(),
		Credit: m.Credit.change(),
	}, nil
}

func (a account) change() domain.ChangeAccount {
	return domain.ChangeAccount{
		ID: a.ID.id(), DebitsPending: a.DebitsPending.amount(), DebitsPosted: a.DebitsPosted.amount(),
		CreditsPending: a.CreditsPending.amount(), CreditsPosted: a.CreditsPosted.amount(),
		UserData128: a.UserData128.id(), UserData64: uint64(a.UserData64), UserData32: a.UserData32,
		Code: a.Code, Flags: a.Flags, Timestamp: uint64(a.Timestamp),
	}
}

// u128 is an unsigned 128-bit integer sent as a JSON number or a decimal string.
type u128 struct{ v ledger.Amount }

func (u *u128) UnmarshalJSON(raw []byte) error {
	digits, err := digitsOf(raw)
	if err != nil {
		return err
	}
	n, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return fmt.Errorf("cdc: %s is not an integer", raw)
	}
	a, err := ledger.AmountFromBig(n)
	if err != nil {
		return fmt.Errorf("cdc: %s does not fit 128 bits", raw)
	}
	u.v = a
	return nil
}

func (u u128) amount() ledger.Amount { return u.v }

// id renders the value as a big-endian ID, the order of the API's IDs.
func (u u128) id() ledger.ID {
	hi, lo := u.v.Parts()
	var id ledger.ID
	for i := range 8 {
		id[i] = byte(hi >> (56 - 8*i))
		id[8+i] = byte(lo >> (56 - 8*i))
	}
	return id
}

// u64 is an unsigned 64-bit integer sent as a JSON number or a decimal string.
type u64 uint64

func (u *u64) UnmarshalJSON(raw []byte) error {
	digits, err := digitsOf(raw)
	if err != nil {
		return err
	}
	v, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return fmt.Errorf("cdc: %s does not fit 64 bits", raw)
	}
	*u = u64(v)
	return nil
}

// digitsOf returns the decimal digits of a JSON number or string, refusing signs, fractions and
// exponents: these values are unsigned integers.
func digitsOf(raw []byte) (string, error) {
	s := raw
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if len(s) == 0 || bytes.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		return "", errors.New("cdc: " + string(raw) + " is not an unsigned integer")
	}
	return string(s), nil
}
