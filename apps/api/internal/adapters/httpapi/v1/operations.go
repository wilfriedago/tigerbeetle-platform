package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
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

// body wraps an answer for Huma, which takes the response body from a field named Body.
type body[T any] struct {
	Body T
}

func registerOperations(api huma.API, h *handlers) {
	tags := []string{"operations"}
	restapi.Register(api, huma.Operation{
		OperationID: "getReconcilerStatus", Method: http.MethodGet, Path: "/v1/_reconciler", Tags: tags,
		Summary: "Reconciler status",
		Description: "The background loop that settles write records left pending by a crash between Postgres\n" +
			"and TigerBeetle (ADR-0007): its last run and the records it is waiting on.\n",
		Responses: restapi.Success(http.StatusOK, "Reconciler status."),
	}, h.getReconcilerStatus)
}

// getReconcilerStatus reports the reconciler's last run and the records it waits on.
func (h *handlers) getReconcilerStatus(ctx context.Context, _ *struct{}) (*body[ReconcilerStatus], error) {
	st, err := h.Reconciler.Status(ctx)
	if err != nil {
		return nil, err
	}
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
	return &body[ReconcilerStatus]{Body: out}, nil
}

// optional returns nil for "", so an absent value is left out of the JSON.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
