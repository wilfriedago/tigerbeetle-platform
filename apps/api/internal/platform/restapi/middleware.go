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

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

// Authenticator checks the credential of a call and resolves the organization it names
// (internal/platform/authn).
type Authenticator interface {
	Authenticate(ctx context.Context, authorization, organization string) (context.Context, error)
}

// Sessions turns the console's session cookie into an access token (ADR-0029); the access
// module implements it.
type Sessions interface {
	AccessToken(ctx context.Context, sessionID string) (string, error)
}

// Options configure Handler.
type Options struct {
	// Auth authenticates every request outside Public.
	Auth Authenticator
	// Public paths need no credential, such as the health probes.
	Public []string
	// Sessions, SessionCookie and ConsoleOrigin let the console authenticate with its session
	// cookie instead of a bearer token; Sessions is nil when the API serves no console.
	Sessions      Sessions
	SessionCookie string
	// ConsoleOrigin is the console's origin, such as http://localhost:5173. A cookie-authenticated
	// request that changes state must come from it (ADR-0029).
	ConsoleOrigin string
}

// Handler wraps mux, on which NewAPI registered the operations, with the standard-library
// middleware every request goes through: request ID, access log, panic recovery, authentication
// and problem+json answers for unknown routes. A body must say it is JSON: anything else answers
// 415, which also keeps cross-site form posts away from cookie sessions (ADR-0031). Huma does no
// routing of its own; the ServeMux does.
func Handler(mux *http.ServeMux, logger *slog.Logger, opts Options) http.Handler {
	h := unknownRoutes(mux)
	h = jsonOnly(h)
	h = authenticate(opts)(h)
	h = recoverPanic(h)
	h = accessLog(h)
	h = requestID(logger)(h)
	return h
}

// OrganizationHeader names the organization a person acts for (ADR-0026).
const OrganizationHeader = "X-Organization"

// authenticate puts the principal and the tenant in the context of every request outside the
// public paths, or answers the problem of a missing or bad credential (ADR-0025). Without an
// Authorization header, the console's session cookie stands in for it (ADR-0029).
func authenticate(opts Options) func(http.Handler) http.Handler {
	open := make(map[string]bool, len(opts.Public))
	for _, p := range opts.Public {
		open[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if open[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			authorization := r.Header.Get("Authorization")
			if authorization == "" && opts.Sessions != nil {
				if c, err := r.Cookie(opts.SessionCookie); err == nil && c.Value != "" {
					if !SameOrigin(r, opts.ConsoleOrigin) {
						WriteProblem(w, r, crossSite)
						return
					}
					token, err := opts.Sessions.AccessToken(r.Context(), c.Value)
					if err != nil {
						WriteProblem(w, r, problem.From(err))
						return
					}
					authorization = "Bearer " + token
				}
			}
			ctx, err := opts.Auth.Authenticate(r.Context(), authorization, r.Header.Get(OrganizationHeader))
			if err != nil {
				WriteProblem(w, r.WithContext(ctx), problem.From(err))
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// crossSite answers a cookie-authenticated request that changes state from another site.
var crossSite = problem.Problem{
	Kind: problem.KindForbidden, Reason: "cross-site-request", Title: problem.TitleFor("cross-site-request"),
	Status: http.StatusForbidden, Detail: "a request that changes state must come from the console", Posting: -1,
}

// SameOrigin reports whether a request may use the session cookie: reads always, since SameSite
// already keeps the cookie off cross-site requests that change state; writes only when the browser
// says the request comes from the same origin (Fetch Metadata), or its Origin is the console's.
// It is the second layer of CSRF defence under SameSite=Lax (ADR-0029).
func SameOrigin(r *http.Request, consoleOrigin string) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if r.Header.Get("Sec-Fetch-Site") == "same-origin" {
		return true
	}
	origin := r.Header.Get("Origin")
	return origin != "" && origin == consoleOrigin
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

// jsonOnly answers 415 to a request whose body does not say it is JSON. Every body of the API is
// JSON; reading a form or text post as JSON would let a page on another site post to the API
// with the console's session cookie (ADR-0031). It runs in front of REST and GraphQL alike.
func jsonOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 && !isJSON(r.Header.Get("Content-Type")) {
			WriteProblem(w, r, problem.Problem{
				Kind: problem.KindInvalid, Reason: "unsupported-media-type", Title: problem.TitleFor("unsupported-media-type"),
				Status: http.StatusUnsupportedMediaType, Detail: "send the body as application/json", Posting: -1,
			})
			return
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
