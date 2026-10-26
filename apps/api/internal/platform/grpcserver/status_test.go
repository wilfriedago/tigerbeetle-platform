package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

func TestStatus(t *testing.T) {
	t.Parallel()
	declined := &ledger.Problem{
		Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "posting 2 would take a below zero",
		Posting: 1, Results: []string{"linked_event_failed", "exceeds_credits"},
	}
	inFlight := &ledger.Problem{Kind: ledger.ErrConflict, Reason: "request-in-flight", Posting: -1, RetryAfter: 1500 * time.Millisecond}
	badPosting := &ledger.Problem{Kind: ledger.ErrInvalid, Reason: "invalid-request", Detail: "posting 3: amount must be positive", Posting: 2}
	tests := []struct {
		name       string
		err        error
		wantCode   codes.Code
		wantMsg    string
		wantReason string
		wantMeta   map[string]string
		wantRetry  time.Duration
		wantField  string
	}{
		{
			name: "declined chain", err: declined, wantCode: codes.FailedPrecondition,
			wantMsg: "posting 2 would take a below zero", wantReason: "EXCEEDS_CREDITS",
			wantMeta: map[string]string{
				"type": problem.TypeOf("exceeds-credits"), "code": "REJECTED", "title": "Declined: exceeds credits",
				"status": "422", "detail": "posting 2 would take a below zero", "requestId": "req-1",
				"posting": "1", "results": "linked_event_failed,exceeds_credits",
			},
		},
		{
			name: "write in flight", err: inFlight, wantCode: codes.Aborted, wantMsg: "Request in flight",
			wantReason: "REQUEST_IN_FLIGHT", wantRetry: 2 * time.Second,
			wantMeta: map[string]string{"type": problem.TypeOf("request-in-flight"), "code": "CONFLICT", "title": "Request in flight", "status": "409", "requestId": "req-1"},
		},
		{
			name: "missing key", err: ledger.NewProblem(ledger.ErrInvalid, "missing-idempotency-key", "every write needs one"),
			wantCode: codes.InvalidArgument, wantMsg: "every write needs one", wantReason: "MISSING_IDEMPOTENCY_KEY",
			wantField: "idempotency_key",
			wantMeta: map[string]string{
				"type": problem.TypeOf("missing-idempotency-key"), "code": "INVALID", "title": "Idempotency-Key required",
				"status": "400", "detail": "every write needs one", "requestId": "req-1",
			},
		},
		{
			name: "invalid posting", err: badPosting, wantCode: codes.InvalidArgument, wantMsg: "posting 3: amount must be positive",
			wantReason: "INVALID_REQUEST", wantField: "postings[2]",
			wantMeta: map[string]string{
				"type": problem.TypeOf("invalid-request"), "code": "INVALID", "title": "Invalid request", "status": "400",
				"detail": "posting 3: amount must be positive", "requestId": "req-1", "posting": "2",
			},
		},
		{
			name: "not found", err: fmt.Errorf("postgres: get ledger: %w", ledger.ErrNotFound), wantCode: codes.NotFound,
			wantMsg: "Not found", wantReason: "NOT_FOUND",
			wantMeta: map[string]string{"type": problem.TypeOf("not-found"), "code": "NOT_FOUND", "title": "Not found", "status": "404", "requestId": "req-1"},
		},
		{
			name: "already exists", err: ledger.ErrAlreadyExists, wantCode: codes.AlreadyExists, wantMsg: "Already exists",
			wantReason: "ALREADY_EXISTS",
			wantMeta:   map[string]string{"type": problem.TypeOf("already-exists"), "code": "ALREADY_EXISTS", "title": "Already exists", "status": "409", "requestId": "req-1"},
		},
		{
			name: "internal", err: errors.New("boom"), wantCode: codes.Internal, wantMsg: "internal error", wantReason: "INTERNAL",
			wantMeta: map[string]string{"type": "about:blank", "code": "INTERNAL", "title": "Internal Server Error", "status": "500", "requestId": "req-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st := Status(tt.err, "req-1")
			if st.Code() != tt.wantCode || st.Message() != tt.wantMsg {
				t.Fatalf("status = %v %q, want %v %q", st.Code(), st.Message(), tt.wantCode, tt.wantMsg)
			}
			info, retry, bad := detailsOf(t, st)
			if info == nil || info.GetReason() != tt.wantReason || info.GetDomain() != ErrorDomain {
				t.Fatalf("ErrorInfo = %v, want reason %s in %s", info, tt.wantReason, ErrorDomain)
			}
			if diff := cmp.Diff(tt.wantMeta, info.GetMetadata()); diff != "" {
				t.Errorf("ErrorInfo metadata (-want +got):\n%s", diff)
			}
			if got := retry.GetRetryDelay().AsDuration(); got != tt.wantRetry {
				t.Errorf("RetryInfo delay = %v, want %v", got, tt.wantRetry)
			}
			gotField := ""
			if v := bad.GetFieldViolations(); len(v) > 0 {
				gotField = v[0].GetField()
			}
			if gotField != tt.wantField {
				t.Errorf("BadRequest field = %q, want %q", gotField, tt.wantField)
			}
		})
	}
}

// TestStatusNoDriverText is the gRPC case of plan section 9.4: the text of a wrapped driver error
// appears in no byte the client receives.
func TestStatusNoDriverText(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		fmt.Errorf("postgres: %w: secret driver text", ledger.ErrUnavailable),
		fmt.Errorf("tigerbeetle: secret driver text: %w", ledger.ErrNotFound),
		errors.New("secret driver text"),
	} {
		raw, marshalErr := proto.Marshal(Status(err, "req-1").Proto())
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if strings.Contains(string(raw), "secret") {
			t.Errorf("status of %q carries the driver text", err)
		}
	}
}

func TestStatusPassesThrough(t *testing.T) {
	t.Parallel()
	own := status.Error(codes.Unimplemented, "unknown method")
	if got := Status(own, "").Code(); got != codes.Unimplemented {
		t.Errorf("gRPC status became %v", got)
	}
	if got := Status(fmt.Errorf("call: %w", context.Canceled), "").Code(); got != codes.Canceled {
		t.Errorf("context.Canceled became %v", got)
	}
	if got := Status(context.DeadlineExceeded, "").Code(); got != codes.DeadlineExceeded {
		t.Errorf("context.DeadlineExceeded became %v", got)
	}
}

func detailsOf(t *testing.T, st *status.Status) (*errdetails.ErrorInfo, *errdetails.RetryInfo, *errdetails.BadRequest) {
	t.Helper()
	var (
		info  *errdetails.ErrorInfo
		retry *errdetails.RetryInfo
		bad   *errdetails.BadRequest
	)
	for _, d := range st.Details() {
		switch v := d.(type) {
		case *errdetails.ErrorInfo:
			info = v
		case *errdetails.RetryInfo:
			retry = v
		case *errdetails.BadRequest:
			bad = v
		default:
			t.Fatalf("unexpected detail %T", d)
		}
	}
	return info, retry, bad
}
