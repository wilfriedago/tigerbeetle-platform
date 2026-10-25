package domain

import (
	"encoding/json"
	"errors"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Stored outcome statuses. The idempotency record keeps "the stored status code and body" so a
// replay answers exactly what the first request answered (ADR-0007). They are numbered like HTTP
// statuses and exposed as responseStatus; no transport derives its own status from them, every
// transport maps from the problem kind (plan section 10).
const (
	StatusCreated     = 201
	StatusOK          = 200
	StatusInvalid     = 400
	StatusConflict    = 409
	StatusRejected    = 422
	StatusInternal    = 500
	StatusUnavailable = 503
)

// storedOutcome is the body kept on a resolved record: the created ID, or the problem.
type storedOutcome struct {
	ID      string   `json:"id,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Detail  string   `json:"detail,omitempty"`
	Posting *int     `json:"posting,omitempty"`
	Results []string `json:"results,omitempty"`
}

var kindNames = map[string]error{
	"invalid":     ledger.ErrInvalid,
	"rejected":    ledger.ErrRejected,
	"conflict":    ledger.ErrConflict,
	"not-found":   ledger.ErrNotFound,
	"unavailable": ledger.ErrUnavailable,
}

func kindName(kind error) string {
	for name, k := range kindNames {
		if errors.Is(kind, k) {
			return name
		}
	}
	return "internal"
}

func statusForKind(kind error) int {
	switch {
	case errors.Is(kind, ledger.ErrInvalid):
		return StatusInvalid
	case errors.Is(kind, ledger.ErrRejected):
		return StatusRejected
	case errors.Is(kind, ledger.ErrConflict):
		return StatusConflict
	case errors.Is(kind, ledger.ErrUnavailable):
		return StatusUnavailable
	default:
		return StatusInternal
	}
}

// ProblemOutcome encodes a problem for the record.
func ProblemOutcome(p *ledger.Problem) (status int, body []byte) {
	o := storedOutcome{Kind: kindName(p.Kind), Reason: p.Reason, Detail: p.Detail, Results: p.Results}
	if p.Posting >= 0 {
		posting := p.Posting
		o.Posting = &posting
	}
	body, _ = json.Marshal(o)
	return statusForKind(p.Kind), body
}

// CommittedBody encodes the outcome of a committed write: the ID it created or resolved.
func CommittedBody(id ledger.ID) []byte {
	body, _ := json.Marshal(storedOutcome{ID: id.String()})
	return body
}

// StoredProblem rebuilds the problem a declined or failed record answered with.
func StoredProblem(rec WriteRecord) error {
	var o storedOutcome
	if err := json.Unmarshal(rec.Response, &o); err != nil || o.Reason == "" {
		return ledger.NewProblem(ledger.ErrRejected, "write-failed", "this Idempotency-Key was used by a write that failed; retry with a new key")
	}
	kind, ok := kindNames[o.Kind]
	if !ok {
		kind = ledger.ErrRejected
	}
	p := &ledger.Problem{Kind: kind, Reason: o.Reason, Detail: o.Detail, Posting: -1, Results: o.Results}
	if o.Posting != nil {
		p.Posting = *o.Posting
	}
	return p
}
