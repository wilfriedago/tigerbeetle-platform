package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// ErrorDomain is the domain of every ErrorInfo the API sends: one fixed service identifier, the
// same string as the service field of GET /v1/_info (ADR-0021, decided R3).
const ErrorDomain = "tigerbeetle-platform-api"

// internalMessage is all a client learns about a fault of ours.
const internalMessage = "internal error"

// Status maps an error to a gRPC status with the standard google.rpc details (plan section 9.2):
//
//   - ErrorInfo always: reason is the problem type's slug in UPPER_SNAKE_CASE (AIP-193), domain is
//     ErrorDomain, and metadata carries the members every other transport sends (type, code,
//     title, status, detail, requestId, posting, results).
//   - RetryInfo when the API asks for a retry later: a write in flight or pending.
//   - BadRequest when an input error names its field: a posting, or the idempotency key.
//
// An error that already is a gRPC status (raised by grpc itself) passes through, and a cancelled
// or expired context keeps its own code.
func Status(err error, requestID string) *status.Status {
	if st, ok := status.FromError(err); ok {
		return st
	}
	switch {
	case errors.Is(err, context.Canceled):
		return status.New(codes.Canceled, "the client cancelled the call")
	case errors.Is(err, context.DeadlineExceeded):
		return status.New(codes.DeadlineExceeded, "the call ran out of time")
	}

	p := problem.From(err)
	code := codeOf(p)
	msg := p.Detail
	if msg == "" {
		msg = p.Title
	}
	if code == codes.Internal {
		msg = internalMessage
	}

	details := []protoadapt.MessageV1{errorInfo(p, requestID)}
	if p.RetryAfter > 0 {
		details = append(details, &errdetails.RetryInfo{RetryDelay: durationpb.New(retryDelay(p))})
	}
	if field := fieldOf(p); field != "" {
		details = append(details, &errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{
			{Field: field, Description: p.Detail, Reason: p.ErrorReason()},
		}})
	}
	st, detailErr := status.New(code, msg).WithDetails(details...)
	if detailErr != nil {
		// Only a message that cannot be marshalled fails here, which the types above rule out.
		return status.New(code, msg)
	}
	return st
}

// codeOf follows the table of plan section 9.2.
func codeOf(p problem.Problem) codes.Code {
	switch p.Kind {
	case problem.KindInvalid:
		return codes.InvalidArgument
	case problem.KindNotFound:
		return codes.NotFound
	case problem.KindAlreadyExists:
		return codes.AlreadyExists
	case problem.KindConflict:
		// A write still in flight is worth retrying as is; every other conflict needs a change.
		if p.Reason == "request-in-flight" {
			return codes.Aborted
		}
		return codes.FailedPrecondition
	case problem.KindRejected:
		return codes.FailedPrecondition
	case problem.KindUnavailable:
		return codes.Unavailable
	default:
		return codes.Internal
	}
}

func errorInfo(p problem.Problem, requestID string) *errdetails.ErrorInfo {
	md := map[string]string{
		"type":   p.Type(),
		"code":   string(p.Kind),
		"title":  p.Title,
		"status": strconv.Itoa(p.Status),
	}
	if p.Detail != "" && p.Kind != problem.KindInternal {
		md["detail"] = p.Detail
	}
	if requestID != "" {
		md["requestId"] = requestID
	}
	if p.Posting >= 0 {
		md["posting"] = strconv.Itoa(p.Posting)
	}
	if len(p.Results) > 0 {
		md["results"] = strings.Join(p.Results, ",")
	}
	return &errdetails.ErrorInfo{Reason: p.ErrorReason(), Domain: ErrorDomain, Metadata: md}
}

// retryDelay rounds up to whole seconds, as the Retry-After header of REST does.
func retryDelay(p problem.Problem) time.Duration {
	return time.Duration(math.Ceil(p.RetryAfter.Seconds())) * time.Second
}

// fieldOf names the request field an input error is about, or "".
func fieldOf(p problem.Problem) string {
	if p.Kind != problem.KindInvalid {
		return ""
	}
	switch {
	case p.Reason == "missing-idempotency-key":
		return "idempotency_key"
	case p.Posting >= 0:
		return fmt.Sprintf("postings[%d]", p.Posting)
	}
	return ""
}
