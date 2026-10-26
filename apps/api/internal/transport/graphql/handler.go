// Package graphql serves the GraphQL API (ADR-0020): the gqlgen executor generated in exec, the
// resolver root built from every module's resolvers, and the HTTP handler with its error presenter
// and limits. It composes modules; modules never import it.
package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/graphql/exec"
)

// Limits on what one request may ask for. A connection's first argument caps its page at 100, so
// complexity counts fields, not rows; the depth limit stops deeply nested selections early.
const (
	maxComplexity = 1000
	maxDepth      = 12
	// maxBodyBytes matches the REST body limit (Huma's default).
	maxBodyBytes = 1 << 20
)

// NewHandler serves POST /graphql. Only the POST transport is enabled; introspection is on, as the
// schema is public in the repository, and the playground is off.
func NewHandler(resolvers exec.ResolverRoot) http.Handler {
	srv := handler.New(exec.NewExecutableSchema(exec.Config{Resolvers: resolvers}))
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(extension.FixedComplexityLimit(maxComplexity))
	srv.Use(depthLimit{max: maxDepth})
	srv.SetErrorPresenter(presentError)
	srv.SetRecoverFunc(func(ctx context.Context, v any) error {
		logging.From(ctx).ErrorContext(ctx, "panic in a GraphQL resolver", slog.Any("panic", v))
		return fmt.Errorf("panic: %v", v)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := requestBody(w, r)
		if !ok {
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		srv.ServeHTTP(w, r)
	})
}

// requestBody reads the body and answers an RFC 9457 problem itself when it is no GraphQL request
// at all: larger than the limit (413), unreadable, or not a JSON object. gqlgen would answer those
// with a GraphQL error that quotes the whole body back; a request that is one goes on to gqlgen,
// whose errors follow the GraphQL format.
func requestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		p := problem.Blank(http.StatusRequestEntityTooLarge)
		p.Detail = fmt.Sprintf("the body is larger than %d bytes", maxBodyBytes)
		restapi.WriteProblem(w, r, p)
		return nil, false
	case err != nil:
		restapi.WriteProblem(w, r, invalidRequest("the body could not be read"))
		return nil, false
	case !isJSONObject(body):
		restapi.WriteProblem(w, r, invalidRequest("the body is not a GraphQL request: a JSON object with a query member"))
		return nil, false
	}
	return body, true
}

func invalidRequest(detail string) problem.Problem {
	return problem.From(ledger.NewProblem(ledger.ErrInvalid, "invalid-request", detail))
}

// isJSONObject reports whether body is one well-formed JSON object.
func isJSONObject(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed)
}

// presentError renders an error with the RFC 9457 members in its extensions (plan section 9.2):
// code, reason, type, title, status, detail, requestId, retryAfter, posting and results, the names
// REST uses, so one client parser reads both. Errors gqlgen raises itself, such as a parse,
// validation or complexity error, are invalid-request problems that keep their code and message.
// No driver text reaches the client: problem.From never copies the text of a wrapped error.
func presentError(ctx context.Context, err error) *gqlerror.Error {
	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) && gqlErr.Err == nil {
		out := *gqlErr
		ext := extensionsOf(ctx, invalidRequest(out.Message))
		for k, v := range out.Extensions {
			ext[k] = v
		}
		out.Extensions = ext
		return &out
	}
	path := graphql.GetPath(ctx)
	cause := err
	if gqlErr != nil {
		cause = gqlErr.Err
		if len(gqlErr.Path) > 0 {
			path = gqlErr.Path
		}
	}
	p := problem.From(cause)
	logProblem(ctx, p, cause)
	return &gqlerror.Error{Message: message(p), Path: path, Extensions: extensionsOf(ctx, p)}
}

func message(p problem.Problem) string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

func extensionsOf(ctx context.Context, p problem.Problem) map[string]any {
	ext := map[string]any{
		"code":   string(p.Kind),
		"type":   p.Type(),
		"title":  p.Title,
		"status": p.Status,
	}
	if p.Reason != "" {
		ext["reason"] = p.Reason
	}
	if p.Detail != "" {
		ext["detail"] = p.Detail
	}
	if id := requestid.From(ctx); id != "" {
		ext["requestId"] = id
	}
	if p.RetryAfter > 0 {
		// Whole seconds, rounded up, as the Retry-After header of REST.
		ext["retryAfter"] = int((p.RetryAfter + time.Second - 1) / time.Second)
	}
	if p.Posting >= 0 {
		ext["posting"] = p.Posting
	}
	if len(p.Results) > 0 {
		ext["results"] = p.Results
	}
	return ext
}

// logProblem logs the full error, which clients never see: at error level for the server's
// faults, at info level for answers such as a decline.
func logProblem(ctx context.Context, p problem.Problem, err error) {
	logger := logging.From(ctx)
	if p.Status >= http.StatusInternalServerError {
		logger.ErrorContext(ctx, "graphql field failed", slog.Int("status", p.Status), slog.Any("error", err))
		return
	}
	logger.InfoContext(ctx, "graphql field rejected", slog.Int("status", p.Status), slog.Any("error", err))
}

// depthLimit refuses an operation whose selections nest deeper than max.
type depthLimit struct {
	max int
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = depthLimit{}

// ExtensionName names the extension in gqlgen's error messages.
func (depthLimit) ExtensionName() string { return "DepthLimit" }

// Validate accepts any schema.
func (depthLimit) Validate(graphql.ExecutableSchema) error { return nil }

// MutateOperationContext measures the operation before it runs.
func (d depthLimit) MutateOperationContext(_ context.Context, op *graphql.OperationContext) *gqlerror.Error {
	if op.Operation == nil {
		return nil
	}
	if depth := selectionDepth(op.Operation.SelectionSet, op.Doc.Fragments, 0); depth > d.max {
		err := gqlerror.Errorf("the operation nests %d levels deep, more than the limit of %d", depth, d.max)
		err.Extensions = map[string]any{"code": "DEPTH_LIMIT_EXCEEDED"}
		return err
	}
	return nil
}

// selectionDepth returns the deepest field nesting of a selection set, following fragments. seen
// guards against a fragment cycle, which validation rejects anyway.
func selectionDepth(set ast.SelectionSet, fragments ast.FragmentDefinitionList, level int) int {
	deepest := level
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			if len(s.SelectionSet) == 0 {
				deepest = max(deepest, level+1)
				continue
			}
			deepest = max(deepest, selectionDepth(s.SelectionSet, fragments, level+1))
		case *ast.InlineFragment:
			deepest = max(deepest, selectionDepth(s.SelectionSet, fragments, level))
		case *ast.FragmentSpread:
			if f := fragments.ForName(s.Name); f != nil && level < 64 {
				deepest = max(deepest, selectionDepth(f.SelectionSet, fragments, level))
			}
		}
	}
	return deepest
}
