package dto

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// WriteRequest is an idempotency record: a write with its key, operation and outcome.
type WriteRequest struct {
	Key            string               `json:"key"`
	Operation      scalar.Operation     `json:"operation"`
	Status         scalar.RequestStatus `json:"status"`
	CreatedAt      time.Time            `json:"createdAt"`
	ResolvedAt     *time.Time           `json:"resolvedAt,omitempty"`
	TargetID       scalar.ID            `json:"targetId" doc:"The transaction or hold minted for this write."`
	ProblemType    *string              `json:"problemType,omitempty" doc:"The problem type of a declined or failed write."`
	ResponseStatus *int                 `json:"responseStatus,omitempty"`
	RequestID      *string              `json:"requestId,omitempty"`
	BodyHash       string               `json:"bodyHash" doc:"SHA-256 of the canonical request body, hex."`
	Attempts       int                  `json:"attempts" doc:"TigerBeetle submissions, including reconciler retries."`
}

// WriteRequestDetail is a write request with its stored request, outcome and minted IDs.
type WriteRequestDetail struct {
	WriteRequest
	Request     map[string]any `json:"request"`
	Response    map[string]any `json:"response,omitempty"`
	TransferIDs []scalar.ID    `json:"transferIds"`
}

// Schema describes WriteRequestDetail as WriteRequest plus the stored request and outcome. The
// extra part is written by hand: the stored bodies are free-form objects.
func (WriteRequestDetail) Schema(r huma.Registry) *huma.Schema {
	return restapi.Component(r, "WriteRequestDetail", func() *huma.Schema {
		return &huma.Schema{AllOf: []*huma.Schema{
			r.Schema(reflect.TypeFor[WriteRequest](), true, ""),
			{
				Type:     huma.TypeObject,
				Required: []string{"request", "transferIds"},
				Properties: map[string]*huma.Schema{
					"request": {Type: huma.TypeObject, Description: "The canonical request body as stored.", AdditionalProperties: true},
					"response": {
						Type: huma.TypeObject, Description: "The stored response or problem body.", AdditionalProperties: true,
					},
					"transferIds": {
						Type: huma.TypeArray, Description: "The transfer IDs minted before the first submission, in chain order.",
						Items: scalar.ID("").Schema(r),
					},
				},
			},
		}}
	})
}

// WriteRequestFrom renders an idempotency record.
func WriteRequestFrom(rec *domain.WriteRecord) WriteRequest {
	w := WriteRequest{
		Key:         rec.Key,
		Operation:   scalar.Operation(rec.Operation),
		Status:      scalar.RequestStatus(rec.Status),
		CreatedAt:   rec.CreatedAt,
		ResolvedAt:  rec.ResolvedAt,
		TargetID:    scalar.IDOf(rec.TargetID),
		ProblemType: restapi.Optional(problem.TypeOf(rec.ProblemType)),
		RequestID:   restapi.Optional(rec.RequestID),
		BodyHash:    hex.EncodeToString(rec.BodyHash),
		Attempts:    rec.Attempts,
	}
	if rec.ResponseStatus != 0 {
		status := rec.ResponseStatus
		w.ResponseStatus = &status
	}
	return w
}

// WriteRequestDetailFrom renders an idempotency record with what it stores.
func WriteRequestDetailFrom(rec *domain.WriteRecord) WriteRequestDetail {
	out := WriteRequestDetail{
		WriteRequest: WriteRequestFrom(rec),
		Request:      jsonObject(rec.Request),
		TransferIDs:  make([]scalar.ID, len(rec.TransferIDs)),
	}
	for i, id := range rec.TransferIDs {
		out.TransferIDs[i] = scalar.IDOf(id)
	}
	if len(rec.Response) > 0 {
		out.Response = jsonObject(rec.Response)
	}
	return out
}

// jsonObject decodes a stored JSON object for display; anything else becomes an empty object.
func jsonObject(raw []byte) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}
