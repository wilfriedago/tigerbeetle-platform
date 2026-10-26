package restapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

// Handler wraps mux, on which NewAPI registered the operations, with the standard-library
// middleware every request goes through: request ID, access log, panic recovery, JSON bodies and
// problem+json answers for unknown routes. Huma does no routing of its own; the ServeMux does.
func Handler(mux *http.ServeMux, logger *slog.Logger) http.Handler {
	h := unknownRoutes(mux)
	h = jsonBodies(h)
	h = recoverPanic(h)
	h = accessLog(h)
	h = requestID(logger)(h)
	return h
}

// pathKey carries the request path, the instance of a problem, to errors built from a context. The
// path identifies the resource the failed request addressed; requestId identifies the occurrence.
type pathKey struct{}

func pathFrom(ctx context.Context) string {
	p, _ := ctx.Value(pathKey{}).(string)
	return p
}

// requestID assigns an ID to every request, keeps a valid client-supplied one, echoes it in the
// response and derives a logger that carries it, so every log line of the request includes it.
func requestID(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestid.Header)
			if !requestid.Valid(id) {
				id = requestid.New()
			}
			w.Header().Set(requestid.Header, id)
			ctx := requestid.With(r.Context(), id)
			ctx = logging.With(ctx, base.With(slog.String("request_id", id)))
			// The escaped form, so instance stays a valid URI reference for a path such as /a%20b.
			ctx = context.WithValue(ctx, pathKey{}, r.URL.EscapedPath())
			// Every call acts for the default organization until authentication resolves the tenant
			// from the credential (docs/plans/auth/plan.md, phase 2; ADR-0026).
			ctx = ledger.WithTenant(ctx, ledger.DefaultTenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// statusRecorder captures the status code and size for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	// 1xx responses are informational; the final status is the first non-1xx code.
	if s.status == 0 && code >= http.StatusOK {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer (flush, deadlines). Huma uses
// it to set the body read deadline.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// accessLog writes one line per request.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		logging.From(r.Context()).InfoContext(r.Context(), "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// recoverPanic turns a panic into a 500 problem instead of killing the connection, and logs the
// stack so the bug is visible.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// http.ErrAbortHandler is net/http's own way to abort a response; pass it on.
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				logging.From(r.Context()).ErrorContext(r.Context(), "panic",
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())))
				WriteProblem(w, r, problem.Blank(http.StatusInternalServerError))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// jsonBodies keeps today's handling of request bodies. Every body is JSON, and the previous
// server decoded it as JSON whatever its Content-Type, so `curl -d '{...}'`, which sends
// application/x-www-form-urlencoded, worked. Huma picks the decoder by Content-Type and would
// answer 415 instead. A stricter answer waits for a new API minor version (plan, question 6).
func jsonBodies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody && !isJSON(r.Header.Get("Content-Type")) {
			r.Header.Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

// isJSON reports whether a Content-Type names JSON: application/json or a +json media type.
func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

// unknownRoutes converts the ServeMux's plain-text 404 and 405 answers into problems. The mux still
// runs so the headers it sets, such as Allow on a 405, are kept.
func unknownRoutes(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		rec := &headerOnlyRecorder{header: w.Header()}
		mux.ServeHTTP(rec, r)
		switch rec.status {
		case http.StatusNotFound:
			WriteProblem(w, r, problem.Blank(http.StatusNotFound))
		case http.StatusMethodNotAllowed:
			WriteProblem(w, r, problem.Blank(http.StatusMethodNotAllowed))
		default:
			// Redirects for path cleaning carry only headers, which are already on w.
			w.WriteHeader(rec.status)
		}
	})
}

// headerOnlyRecorder shares the real header map but drops the body, so the mux's text/plain error
// body is replaced while its headers survive.
type headerOnlyRecorder struct {
	header http.Header
	status int
}

func (h *headerOnlyRecorder) Header() http.Header { return h.header }

func (h *headerOnlyRecorder) WriteHeader(code int) {
	if h.status == 0 {
		h.status = code
	}
}

func (h *headerOnlyRecorder) Write(b []byte) (int, error) {
	if h.status == 0 {
		h.status = http.StatusOK
	}
	return len(b), nil
}

// WriteProblem answers with a problem made outside Huma: by the middleware, or by a handler on the
// same mux such as POST /graphql when the body is not a GraphQL request at all.
func WriteProblem(w http.ResponseWriter, r *http.Request, p problem.Problem) {
	e := NewError(r.Context(), p)
	for k, v := range e.GetHeaders() {
		w.Header()[k] = v
	}
	w.Header().Set("Content-Type", problemMediaType)
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(e); err != nil {
		logging.From(r.Context()).ErrorContext(r.Context(), "write problem response", slog.Any("error", err))
	}
}
