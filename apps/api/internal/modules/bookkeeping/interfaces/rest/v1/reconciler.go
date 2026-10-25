package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

func registerReconciler(api huma.API, h *handlers) {
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
func (h *handlers) getReconcilerStatus(ctx context.Context, _ *struct{}) (*restapi.Body[dto.ReconcilerStatus], error) {
	st, err := h.Reconciler.Status(ctx)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.ReconcilerStatus]{Body: dto.ReconcilerStatusFrom(&st)}, nil
}
