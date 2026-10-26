package restapi

import (
	"context"
	"reflect"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

// Register adds one operation to api. Every operation documents the shared Problem response as its
// default, and every handler error goes through Fail, so no TigerBeetle or Postgres text reaches a
// client.
//
// Validation stays in the domain (ADR-0018): Huma decodes parameters and bodies but validates
// neither, so REST answers input errors the way every other transport does, and the constraints
// in the spec document what the domain checks. Huma still reports values it cannot decode, such
// as limit=abc, and a missing required body.
func Register[I, O any](api huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	// Every operation needs a bearer token (ADR-0026) and may name the organization it acts for
	// (ADR-0027), except the public ones registered with RegisterPublic.
	op.Security = []map[string][]string{{SecurityScheme: {}}}
	op.Parameters = append(op.Parameters, organizationParam())
	register(api, op, handler)
}

// RegisterPlatform adds an operation that needs a bearer token but acts for no organization, such
// as the staff's organization operations (ADR-0027): it documents no X-Organization header.
func RegisterPlatform[I, O any](api huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	op.Security = []map[string][]string{{SecurityScheme: {}}}
	register(api, op, handler)
}

// RegisterPublic adds an operation that needs no credential: the health probes, which
// orchestrators and load balancers call. The middleware's public paths must list its path too.
func RegisterPublic[I, O any](api huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	register(api, op, handler)
}

func register[I, O any](api huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	if op.Responses == nil {
		op.Responses = map[string]*huma.Response{}
	}
	for _, r := range op.Responses {
		if r.Headers == nil {
			r.Headers = map[string]*huma.Param{}
		}
		r.Headers[requestid.Header] = requestIDResponseHeader()
	}
	op.Responses["default"] = &huma.Response{Ref: problemResponseRef}
	op.Parameters = append(op.Parameters, requestIDParam())
	op.SkipValidateParams = true
	op.SkipValidateBody = true
	huma.Register(api, op, func(ctx context.Context, in *I) (*O, error) {
		out, err := handler(ctx, in)
		if err != nil {
			return nil, Fail(ctx, err)
		}
		return out, nil
	})
}

// Success documents the answer of an operation by its status and description. Huma fills in the
// body schema from the operation's output type.
func Success(status int, description string) map[string]*huma.Response {
	return map[string]*huma.Response{strconv.Itoa(status): {Description: description}}
}

// ProblemWith documents an answer that is a problem with the extension members of T, such as the
// 503 of /readyz, which adds the dependencies to the Problem schema.
func ProblemWith[T any](api huma.API, description string) *huma.Response {
	registry := api.OpenAPI().Components.Schemas
	members := registry.Schema(reflect.TypeFor[T](), true, "")
	return &huma.Response{
		Description: description,
		Headers:     map[string]*huma.Param{requestid.Header: requestIDResponseHeader()},
		Content: map[string]*huma.MediaType{problemMediaType: {Schema: &huma.Schema{
			AllOf: []*huma.Schema{(*Error)(nil).Schema(registry), members},
		}}},
	}
}

// organizationParam documents the X-Organization request header.
func organizationParam() *huma.Param {
	maxLength := 63
	return &huma.Param{
		Name: OrganizationHeader, In: "header",
		Description: "The alias of the organization the call acts for. A person sends it on every call about a " +
			"ledger and must be a member of the organization; an organization that is not one of theirs answers " +
			"404. API keys and machine clients belong to one organization and need not send it.",
		Schema: &huma.Schema{Type: huma.TypeString, MaxLength: &maxLength},
	}
}

// requestIDParam documents the X-Request-Id request header the middleware reads on every request.
func requestIDParam() *huma.Param {
	maxLength := 128
	return &huma.Param{
		Name: requestid.Header, In: "header",
		Description: "Correlates the request with the service logs. Kept when it is 1 to 128 printable ASCII " +
			"characters without spaces, replaced by a new ID otherwise, and returned in the X-Request-Id " +
			"response header.",
		Schema: &huma.Schema{Type: huma.TypeString, MaxLength: &maxLength},
	}
}

// requestIDResponseHeader documents the X-Request-Id header of every answer.
func requestIDResponseHeader() *huma.Param {
	return &huma.Param{
		Description: "The request ID: the one the client sent when it was valid, a new one otherwise.",
		Schema:      &huma.Schema{Type: huma.TypeString},
	}
}
