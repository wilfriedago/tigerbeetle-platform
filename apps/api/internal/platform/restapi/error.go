package restapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// problemMediaType is the RFC 9457 media type of every error body.
const problemMediaType = "application/problem+json"

// Error is a problem on its way to a REST client: the transport-neutral problem plus the request it
// answers. It implements Huma's error interfaces, so a handler returns it like any error and Huma
// writes it with its status, headers and media type.
type Error struct {
	Problem   problem.Problem
	Instance  string
	RequestID string
}

// Fail turns any error into a REST problem for the request in ctx and logs the cause: at error
// level when the fault is ours (5xx), at info level when the client's request was refused. The
// cause's text goes to the log only.
func Fail(ctx context.Context, err error) *Error {
	var re *Error
	if errors.As(err, &re) {
		return re
	}
	e := newError(ctx, problem.From(err))
	logger := logging.From(ctx)
	if e.Problem.Status >= http.StatusInternalServerError {
		logger.ErrorContext(ctx, "request failed", slog.Int("status", e.Problem.Status), slog.Any("error", err))
	} else {
		logger.InfoContext(ctx, "request rejected", slog.Int("status", e.Problem.Status), slog.Any("error", err))
	}
	return e
}

// newError stamps a problem with the request ID and path of the request in ctx.
func newError(ctx context.Context, p problem.Problem) *Error {
	return &Error{Problem: p, Instance: pathFrom(ctx), RequestID: requestid.From(ctx)}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d %s", e.Problem.Status, e.Problem.Type())
}

// GetStatus is the HTTP status of the answer.
func (e *Error) GetStatus() int {
	return e.Problem.Status
}

// ContentType makes Huma send problems as application/problem+json.
func (*Error) ContentType(string) string {
	return problemMediaType
}

// GetHeaders adds Retry-After when the problem suggests a delay, in whole seconds rounded up, and
// stops browsers from sniffing the body as something else.
func (e *Error) GetHeaders() http.Header {
	h := http.Header{}
	h.Set("X-Content-Type-Options", "nosniff")
	if d := e.Problem.RetryAfter; d > 0 {
		h.Set("Retry-After", strconv.Itoa(int((d+time.Second-1)/time.Second)))
	}
	return h
}

// problemHeaders sets the headers of a problem on every problem Huma writes. Huma applies them
// itself only to errors a handler returns, not to the errors it raises while decoding a request.
func problemHeaders(ctx huma.Context, _ string, v any) (any, error) {
	if e, ok := v.(*Error); ok {
		for name, values := range e.GetHeaders() {
			ctx.SetHeader(name, values[0])
		}
	}
	return v, nil
}

// problemBody is the JSON form of a problem, with the members of the Problem schema.
type problemBody struct {
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Status    int      `json:"status"`
	Detail    string   `json:"detail,omitempty"`
	Instance  string   `json:"instance,omitempty"`
	RequestID string   `json:"requestId,omitempty"`
	Posting   *int     `json:"posting,omitempty"`
	Results   []string `json:"results,omitempty"`
}

// MarshalJSON writes the RFC 9457 body. Posting is a pointer because 0 is a valid index.
func (e *Error) MarshalJSON() ([]byte, error) {
	p := e.Problem
	body := problemBody{
		Type: p.Type(), Title: p.Title, Status: p.Status, Detail: p.Detail,
		Instance: e.Instance, RequestID: e.RequestID, Results: p.Results,
	}
	if p.Posting >= 0 {
		posting := p.Posting
		body.Posting = &posting
	}
	return json.Marshal(body)
}

// newHumaError and newHumaErrorWithContext replace Huma's error model, so the errors Huma raises
// itself (an unparsable parameter or body, a missing required body) take the same shape as
// handler errors. Input errors become 400 invalid-request with Huma's messages as the detail, as
// the domain answers today; anything else is an internal error with no detail.
func newHumaError(status int, msg string, errs ...error) huma.StatusError {
	return humaProblem(context.Background(), status, msg, errs)
}

func newHumaErrorWithContext(hctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
	ctx := context.Background()
	if hctx != nil {
		ctx = hctx.Context()
	}
	return humaProblem(ctx, status, msg, errs)
}

func humaProblem(ctx context.Context, status int, msg string, errs []error) *Error {
	if status >= http.StatusInternalServerError {
		logging.From(ctx).ErrorContext(ctx, "request failed", slog.Int("status", status), slog.String("message", msg), slog.Any("errors", errs))
		return newError(ctx, problem.Blank(http.StatusInternalServerError, "Internal server error"))
	}
	p := problem.Problem{
		Kind: problem.KindInvalid, Reason: "invalid-request", Title: problem.TitleFor("invalid-request"),
		Status: http.StatusBadRequest, Detail: humaDetail(msg, errs), Posting: -1,
	}
	return newError(ctx, p)
}

// humaDetail joins Huma's error details as "location: message". They describe the client's own
// input, so they are safe to return.
func humaDetail(msg string, errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		var d *huma.ErrorDetail
		switch {
		case errors.As(err, &d) && d.Location != "":
			parts = append(parts, d.Location+": "+d.Message)
		case errors.As(err, &d):
			parts = append(parts, d.Message)
		case err != nil:
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return msg
	}
	return strings.Join(parts, "; ")
}

// Schema describes the Problem component of the spec. Huma asks for it when it registers the error
// model; it is written by hand to keep the RFC 9457 keywords the generic schema would lose.
func (*Error) Schema(r huma.Registry) *huma.Schema {
	if _, ok := r.Map()["Problem"]; !ok {
		statusMin, statusMax := 100.0, 599.0
		targetID := scalar.ID("").Schema(r)
		targetID.Description = "The transaction or hold ID minted for a declined write."
		r.Map()["Problem"] = &huma.Schema{
			Type:        huma.TypeObject,
			Description: "RFC 9457 problem details.",
			Required:    []string{"type", "title", "status"},
			Properties: map[string]*huma.Schema{
				"type": {
					Type: huma.TypeString, Format: "uri-reference", Default: problem.TypeBlank,
					Description: "A URI reference that identifies the problem type.",
				},
				"title":  {Type: huma.TypeString, Description: "A short, human-readable summary of the problem type."},
				"status": {Type: huma.TypeInteger, Minimum: &statusMin, Maximum: &statusMax, Description: "The HTTP status code."},
				"detail": {Type: huma.TypeString, Description: "A human-readable explanation specific to this occurrence."},
				"instance": {
					Type: huma.TypeString, Description: "A URI reference that identifies the specific occurrence.",
				},
				"requestId": {Type: huma.TypeString, Description: "The request ID, also returned in the X-Request-Id header."},
				"posting": {
					Type: huma.TypeInteger, Description: "The zero-based index of the posting that caused the problem.",
				},
				"results": {
					Type: huma.TypeArray, Items: &huma.Schema{Type: huma.TypeString},
					Description: "The TigerBeetle result of every posting of a declined chain, in order, such as\n" +
						"`[\"linked_event_failed\", \"exceeds_credits\"]`. Nothing was applied.\n",
				},
				"targetId": targetID,
			},
		}
	}
	return &huma.Schema{Ref: "#/components/schemas/Problem"}
}

// problemResponseRef is the shared error response every operation documents as its default.
const problemResponseRef = "#/components/responses/Problem"

// registerComponents adds the shared Problem response, which every operation's default response
// refers to.
func registerComponents(oapi *huma.OpenAPI) {
	if oapi.Components.Responses == nil {
		oapi.Components.Responses = map[string]*huma.Response{}
	}
	oapi.Components.Responses["Problem"] = &huma.Response{
		Description: "An error described as RFC 9457 problem details.",
		Content: map[string]*huma.MediaType{
			problemMediaType: {Schema: (*Error)(nil).Schema(oapi.Components.Schemas)},
		},
	}
}
