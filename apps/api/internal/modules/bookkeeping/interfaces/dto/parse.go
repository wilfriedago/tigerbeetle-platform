package dto

import (
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// ParsePostings parses the postings of a write. A posting that does not parse is an invalid
// request that names the posting, with the same detail on every transport.
func ParsePostings(in []Posting) ([]domain.Posting, error) {
	out := make([]domain.Posting, len(in))
	for i := range in {
		posting, err := in[i].Parse()
		if err != nil {
			return nil, &ledger.Problem{
				Kind: ledger.ErrInvalid, Reason: "invalid-request", Posting: i,
				Detail: "posting " + strconv.Itoa(i+1) + ": " + problem.From(err).Detail,
			}
		}
		out[i] = posting
	}
	return out, nil
}

// Parse parses one posting: two addresses, an asset and an amount.
func (p Posting) Parse() (domain.Posting, error) {
	src, err := ledger.ParseAddress(string(p.Source))
	if err != nil {
		return domain.Posting{}, err
	}
	dst, err := ledger.ParseAddress(string(p.Destination))
	if err != nil {
		return domain.Posting{}, err
	}
	a, err := ledger.ParseAsset(string(p.Asset))
	if err != nil {
		return domain.Posting{}, err
	}
	amt, err := ledger.ParseAmount(string(p.Amount))
	if err != nil {
		return domain.Posting{}, err
	}
	return domain.Posting{Source: src, Destination: dst, Asset: a, Amount: amt}, nil
}
