package dto

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// StuckRequest is a pending write record the reconciler could not settle.
type StuckRequest struct {
	Ledger    scalar.LedgerName `json:"ledger"`
	Key       string            `json:"key"`
	Operation scalar.Operation  `json:"operation"`
	CreatedAt time.Time         `json:"createdAt"`
	Attempts  int               `json:"attempts"`
	Note      *string           `json:"note,omitempty"`
}

// ReconcilerStatus is the answer of GET /v1/_reconciler.
type ReconcilerStatus struct {
	Running            bool       `json:"running"`
	GracePeriodSeconds int        `json:"gracePeriodSeconds" doc:"A pending record younger than this is a write in progress, not a crash."`
	IntervalSeconds    int        `json:"intervalSeconds"`
	LastRunAt          *time.Time `json:"lastRunAt,omitempty"`
	LastSettled        *int       `json:"lastSettled,omitempty" doc:"Records settled by the last run."`
	Pending            int64      `json:"pending" doc:"Pending records, across all ledgers."`
	Stuck              int64      `json:"stuck" doc:"Pending records older than the grace period that the reconciler could not settle."`
	// StuckRequests is always written, as an empty list when nothing is stuck.
	StuckRequests []StuckRequest `json:"stuckRequests" required:"false"`
}

// ReconcilerStatusFrom renders the reconciler's last run and the records it waits on.
func ReconcilerStatusFrom(st *application.ReconcilerStatus) ReconcilerStatus {
	out := ReconcilerStatus{
		Running:            st.Running,
		GracePeriodSeconds: int(st.Grace / time.Second),
		IntervalSeconds:    int(st.Interval / time.Second),
		Pending:            st.Pending,
		Stuck:              st.Stuck,
		StuckRequests:      make([]StuckRequest, 0, len(st.Records)),
	}
	if !st.LastRunAt.IsZero() {
		at := st.LastRunAt.UTC()
		settled := st.LastSettled
		out.LastRunAt, out.LastSettled = &at, &settled
	}
	for _, r := range st.Records {
		note := r.Note
		out.StuckRequests = append(out.StuckRequests, StuckRequest{
			Ledger: scalar.LedgerName(r.Ledger), Key: r.Key, Operation: scalar.Operation(r.Operation),
			CreatedAt: r.CreatedAt, Attempts: r.Attempts, Note: &note,
		})
	}
	return out
}
