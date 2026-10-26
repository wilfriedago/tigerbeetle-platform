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
	// Errors lists the parameters and body members Huma could not decode, one entry each: the
	// errors extension of the RFC 9457 validation example.
	Errors []InputError
	// Extensions holds the extension members one operation adds, such as the dependencies of a
	// 503 from /readyz. A name the Problem schema already uses is refused when the body is written.
	Extensions map[string]any
}

// InputError is one entry of the errors member: what is wrong, and where. Pointer is a JSON
// Pointer into the body in URI fragment form ("#/postings/0/amount", "#" for the whole body);
// Parameter and In name a path, query or header parameter.
type InputError struct {
	Detail    string `json:"detail"`
	Pointer   string `json:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	In        string `json:"in,omitempty"`
}

// Fail turns any error into a REST problem for the request in ctx and logs the cause: at error
// level when the fault is ours (5xx), at info level when the client's request was refused. The
// cause's text goes to the log only.
func Fail(ctx context.Context, err error) *Error {
	var re *Error
	if errors.As(err, &re) {
		return re
	}
	e := NewError(ctx, problem.From(err))
	logger := logging.From(ctx)
	if e.Problem.Status >= http.StatusInternalServerError {
		logger.ErrorContext(ctx, "request failed", slog.Int("status", e.Problem.Status), slog.Any("error", err))
	} else {
		logger.InfoContext(ctx, "request rejected", slog.Int("status", e.Problem.Status), slog.Any("error", err))
	}
	return e
}

// NewError stamps a problem with the request ID and path of the request in ctx. It logs nothing,
// for an operation whose contract answers a problem, such as /readyz when a dependency is down.
func NewError(ctx context.Context, p problem.Problem) *Error {
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
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	RequestID string       `json:"requestId,omitempty"`
	Posting   *int         `json:"posting,omitempty"`
	Results   []string     `json:"results,omitempty"`
	Errors    []InputError `json:"errors,omitempty"`
}

// reservedMembers are the names of problemBody, which an extension member may not reuse.
var reservedMembers = map[string]bool{
	"type": true, "title": true, "status": true, "detail": true, "instance": true,
	"requestId": true, "posting": true, "results": true, "errors": true, "targetId": true,
}

// MarshalJSON writes the RFC 9457 body. Posting is a pointer because 0 is a valid index.
// Extension members follow the standard ones in the same object.
func (e *Error) MarshalJSON() ([]byte, error) {
	p := e.Problem
	body := problemBody{
		Type: p.Type(), Title: p.Title, Status: p.Status, Detail: p.Detail,
		Instance: e.Instance, RequestID: e.RequestID, Results: p.Results, Errors: e.Errors,
	}
	if p.Posting >= 0 {
		posting := p.Posting
		body.Posting = &posting
	}
	out, err := json.Marshal(body)
	if err != nil || len(e.Extensions) == 0 {
		return out, err
	}
	for name := range e.Extensions {
		if reservedMembers[name] {
			return nil, fmt.Errorf("problem extension %q reuses a member of the Problem schema", name)
		}
	}
	ext, err := json.Marshal(e.Extensions)
	if err != nil {
		return nil, err
	}
	// Both are JSON objects: drop the closing brace of one and the opening brace of the other.
	return append(append(out[:len(out)-1], ','), ext[1:]...), nil
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
		return NewError(ctx, problem.Blank(http.StatusInternalServerError))
	}
	inputs := inputErrors(errs)
	var p problem.Problem
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		// What the client sent could not be decoded: the same answer as the domain's input errors.
		p = problem.Problem{
			Kind: problem.KindInvalid, Reason: "invalid-request", Title: problem.TitleFor("invalid-request"),
			Status: http.StatusBadRequest, Detail: humaDetail(msg, inputs), Posting: -1,
		}
	default:
		// Anything else Huma refuses, such as a body over the size limit (413), keeps its status.
		p = problem.Blank(status)
		p.Detail = msg
	}
	e := NewError(ctx, p)
	e.Errors = inputs
	return e
}

// inputErrors turns Huma's error details into entries of the errors member. Huma writes the
// location as "query.limit", "path.name", "header.Idempotency-Key" or "body"; a body error
// carries its own pointer (see decodeJSON). The messages describe the client's own input, so they
// are safe to return.
func inputErrors(errs []error) []InputError {
	out := make([]InputError, 0, len(errs))
	for _, err := range errs {
		var d *huma.ErrorDetail
		if !errors.As(err, &d) {
			if err != nil {
				out = append(out, InputError{Detail: err.Error()})
			}
			continue
		}
		in, name, _ := strings.Cut(d.Location, ".")
		switch in {
		case "body":
			pointer, detail := splitBodyMessage(d.Message)
			out = append(out, InputError{Detail: detail, Pointer: pointer})
		case "path", "query", "header", "cookie":
			out = append(out, InputError{Detail: d.Message, Parameter: name, In: in})
		default:
			out = append(out, InputError{Detail: d.Message})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// humaDetail joins the input errors as "where: message"; without any, Huma's message is the
// detail.
func humaDetail(msg string, inputs []InputError) string {
	parts := make([]string, 0, len(inputs))
	for _, in := range inputs {
		switch {
		case in.Pointer != "":
			parts = append(parts, "body"+strings.TrimPrefix(in.Pointer, "#")+": "+in.Detail)
		case in.Parameter != "":
			parts = append(parts, in.In+"."+in.Parameter+": "+in.Detail)
		default:
			parts = append(parts, in.Detail)
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
					Description: "An absolute URI that identifies the problem type: `about:blank`, or the entry of\n" +
						"the problem in the catalogue, the same in every deployment, which clients switch on:\n" +
						"`" + problem.TypeBase + "<reason>`.\n",
					Examples: []any{problem.TypeOf("exceeds-credits")},
				},
				"title": {
					Type:        huma.TypeString,
					Description: "A short summary of the problem type, the same for every occurrence. For `about:blank` it is the HTTP status phrase.",
				},
				"status": {Type: huma.TypeInteger, Minimum: &statusMin, Maximum: &statusMax, Description: "The HTTP status code."},
				"detail": {Type: huma.TypeString, Description: "A human-readable explanation specific to this occurrence."},
				"instance": {
					Type: huma.TypeString, Format: "uri-reference",
					Description: "The path of the request that failed, a URI reference relative to the API.",
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
				"errors": {
					Type: huma.TypeArray, Items: inputErrorSchema(),
					Description: "Each parameter or body member that could not be decoded, for an `invalid-request` problem.",
				},
				"targetId": targetID,
			},
		}
	}
	return &huma.Schema{Ref: "#/components/schemas/Problem"}
}

// inputErrorSchema describes one entry of the errors member.
func inputErrorSchema() *huma.Schema {
	return &huma.Schema{
		Type:     huma.TypeObject,
		Required: []string{"detail"},
		Properties: map[string]*huma.Schema{
			"detail": {Type: huma.TypeString, Description: "What is wrong with the value."},
			"pointer": {
				Type: huma.TypeString, Format: "uri-reference",
				Description: "A JSON Pointer in URI fragment form to the body member at fault, such as `#/postings/0/amount`; `#` is the whole body.",
			},
			"parameter": {Type: huma.TypeString, Description: "The name of the parameter at fault."},
			"in": {
				Type: huma.TypeString, Enum: []any{"path", "query", "header", "cookie"},
				Description: "Where the parameter is.",
			},
		},
	}
}

// problemResponseRef is the shared error response every operation documents as its default.
const problemResponseRef = "#/components/responses/Problem"

// registerComponents adds the shared Problem response, which every operation's default response
// refers to.
func registerComponents(oapi *huma.OpenAPI) {
	if oapi.Components.Responses == nil {
		oapi.Components.Responses = map[string]*huma.Response{}
	}
	retryMinimum := 1.0
	oapi.Components.Responses["Problem"] = &huma.Response{
		Description: "An error described as RFC 9457 problem details.",
		Headers: map[string]*huma.Param{
			requestid.Header: requestIDResponseHeader(),
			"Retry-After": {
				Description: "Seconds to wait before retrying, sent with a 409 `request-in-flight` and a 503 " +
					"`write-pending`.",
				Schema: &huma.Schema{Type: huma.TypeInteger, Minimum: &retryMinimum},
			},
		},
		Content: map[string]*huma.MediaType{
			problemMediaType: {Schema: (*Error)(nil).Schema(oapi.Components.Schemas)},
		},
	}
}
