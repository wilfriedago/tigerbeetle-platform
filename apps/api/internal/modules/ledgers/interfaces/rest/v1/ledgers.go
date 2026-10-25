package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

func registerLedgers(api huma.API, h *handlers) {
	tags := []string{"ledgers"}
	restapi.Register(api, huma.Operation{
		OperationID: "listLedgers", Method: http.MethodGet, Path: "/v1/ledgers", Tags: tags,
		Summary:     "List ledgers",
		Description: "Every ledger in name order, with its assets and account count.",
		Parameters:  []*huma.Param{restapi.CursorParam()},
		Responses:   restapi.Success(http.StatusOK, "A page of ledgers."),
	}, h.listLedgers)

	restapi.Register(api, huma.Operation{
		OperationID: "getLedger", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}", Tags: tags,
		Summary:     "Get a ledger",
		Description: "The ledger with its metadata, assets and account count.",
		Responses:   restapi.Success(http.StatusOK, "The ledger."),
	}, h.getLedger)

	restapi.Register(api, huma.Operation{
		OperationID: "createLedger", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}", Tags: tags,
		Summary: "Create a ledger",
		Description: "Creates a ledger. The name is permanent and appears in every API path. A ledger cannot be\n" +
			"deleted, because TigerBeetle accounts and transfers are permanent. Answers 409 when the\n" +
			"name is taken.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The ledger was created."),
	}, h.createLedger)

	restapi.Register(api, huma.Operation{
		OperationID: "replaceLedgerMetadata", Method: http.MethodPut, Path: "/v1/ledgers/{ledger}/metadata", Tags: tags,
		Summary:     "Replace ledger metadata",
		Description: "Replaces the metadata of the ledger with the given key/value pairs.",
		Responses:   restapi.Success(http.StatusOK, "The ledger with its new metadata."),
	}, h.replaceLedgerMetadata)
}

func (h *handlers) listLedgers(ctx context.Context, in *struct{ restapi.Page }) (*restapi.Body[dto.LedgerList], error) {
	views, next, err := h.LedgerQueries.List(ctx, in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	out := dto.LedgerList{Data: make([]dto.Ledger, len(views)), Next: restapi.Optional(next)}
	for i, v := range views {
		out.Data[i] = dto.LedgerFrom(v)
	}
	return &restapi.Body[dto.LedgerList]{Body: out}, nil
}

func (h *handlers) getLedger(ctx context.Context, in *restapi.LedgerPath) (*restapi.Body[dto.Ledger], error) {
	v, err := h.LedgerQueries.Get(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Ledger]{Body: dto.LedgerFrom(v)}, nil
}

func (h *handlers) createLedger(ctx context.Context, in *struct {
	restapi.LedgerPath
	Body dto.CreateLedgerRequest
},
) (*restapi.Body[dto.Ledger], error) {
	v, err := h.LedgerCommands.Create(ctx, string(in.Ledger), in.Body.Metadata.Map())
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Ledger]{Body: dto.LedgerFrom(v)}, nil
}

func (h *handlers) replaceLedgerMetadata(ctx context.Context, in *struct {
	restapi.LedgerPath
	Body scalar.Metadata
},
) (*restapi.Body[dto.Ledger], error) {
	v, err := h.LedgerCommands.ReplaceMetadata(ctx, string(in.Ledger), in.Body.Map())
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Ledger]{Body: dto.LedgerFrom(v)}, nil
}
