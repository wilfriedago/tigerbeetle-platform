package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

type ledgerParams struct {
	Ledger string `json:"ledger"`
}

type createBody struct {
	Amount string `json:"amount"`
}

func newDispatcher(t *testing.T, ran *atomic.Int32) *Dispatcher {
	t.Helper()
	r := NewRouter()
	r.Handle("v1.echo", func(ctx context.Context, req Request) (any, error) {
		ran.Add(1)
		p, err := Params[ledgerParams](req)
		if err != nil {
			return nil, err
		}
		b, err := Body[createBody](req)
		if err != nil {
			return nil, err
		}
		dl, _ := ctx.Deadline()
		return map[string]any{
			"ledger": p.Ledger, "amount": b.Amount, "key": req.IdempotencyKey,
			"requestId": requestid.From(ctx), "deadline": dl.UnixMilli(),
		}, nil
	})
	r.Handle("v1.fail", func(context.Context, Request) (any, error) {
		ran.Add(1)
		return nil, &ledger.Problem{Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "no money", Posting: 1, Results: []string{"linked_event_failed", "exceeds_credits"}}
	})
	r.Handle("v1.leak", func(context.Context, Request) (any, error) {
		ran.Add(1)
		return nil, fmt.Errorf("postgres: %w: secret driver text", ledger.ErrUnavailable)
	})
	r.Handle("v1.panic", func(context.Context, Request) (any, error) {
		ran.Add(1)
		panic("secret panic text")
	})
	d := NewDispatcher(r, 30*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.now = func() time.Time { return time.UnixMilli(1_000_000) }
	return d
}

func TestDispatcher(t *testing.T) {
	t.Parallel()
	posting := 1
	tests := []struct {
		name    string
		call    Call
		wantRan int32
		want    Reply
	}{
		{
			name: "ok, with the client's earlier deadline",
			call: Call{
				Method: "v1.echo", Payload: []byte(`{"params":{"ledger":"eu"},"body":{"amount":"5"}}`),
				RequestID: "client-1", IdempotencyKey: "k1", Deadline: "1005000",
			},
			wantRan: 1,
			want: Reply{OK: true, Data: map[string]any{
				"ledger": "eu", "amount": "5", "key": "k1", "requestId": "client-1", "deadline": float64(1_005_000),
			}},
		},
		{
			name:    "ok, the server's limit when the client's deadline is later",
			call:    Call{Method: "v1.echo", RequestID: "client-1", Deadline: "9999999999"},
			wantRan: 1,
			want: Reply{OK: true, Data: map[string]any{
				"ledger": "", "amount": "", "key": "", "requestId": "client-1", "deadline": float64(1_030_000),
			}},
		},
		{
			name:    "a deadline already passed is not run",
			call:    Call{Method: "v1.echo", RequestID: "client-1", Deadline: "999999"},
			wantRan: 0,
			want: Reply{Error: &Error{
				Code: "UNAVAILABLE", Reason: "deadline-exceeded", Type: "/problems/deadline-exceeded", Title: "Deadline exceeded",
				Status: 503, Detail: "the deadline passed before the request ran; it was not executed", RequestID: "client-1",
			}},
		},
		{
			name: "unknown method",
			call: Call{Method: "v1.nope", RequestID: "client-1"},
			want: Reply{Error: &Error{
				Code: "NOT_FOUND", Reason: "unknown-method", Type: "/problems/unknown-method", Title: "Unknown method",
				Status: 404, Detail: `no method "v1.nope"`, RequestID: "client-1",
			}},
		},
		{
			name: "malformed envelope",
			call: Call{Method: "v1.echo", Payload: []byte(`{"params":`), RequestID: "client-1"},
			want: Reply{Error: &Error{
				Code: "INVALID", Reason: "invalid-request", Type: "/problems/invalid-request", Title: "Invalid request",
				Status: 400, Detail: "the message is not an RPC envelope: unexpected EOF", RequestID: "client-1",
			}},
		},
		{
			name:    "unknown parameter",
			call:    Call{Method: "v1.echo", Payload: []byte(`{"params":{"ledgr":"eu"}}`), RequestID: "client-1"},
			wantRan: 1,
			want: Reply{Error: &Error{
				Code: "INVALID", Reason: "invalid-request", Type: "/problems/invalid-request", Title: "Invalid request",
				Status: 400, Detail: `params: json: unknown field "ledgr"`, RequestID: "client-1",
			}},
		},
		{
			name:    "a declined chain keeps posting and results",
			call:    Call{Method: "v1.fail", RequestID: "client-1"},
			wantRan: 1,
			want: Reply{Error: &Error{
				Code: "REJECTED", Reason: "exceeds-credits", Type: "/problems/exceeds-credits", Title: "Declined: exceeds credits",
				Status: 422, Detail: "no money", RequestID: "client-1", Posting: &posting, Results: []string{"linked_event_failed", "exceeds_credits"},
			}},
		},
		{
			name:    "driver text stays in the log",
			call:    Call{Method: "v1.leak", RequestID: "client-1"},
			wantRan: 1,
			want: Reply{Error: &Error{
				Code: "UNAVAILABLE", Reason: "dependency-unavailable", Type: "/problems/dependency-unavailable",
				Title: "Dependency unavailable", Status: 503, RequestID: "client-1",
			}},
		},
		{
			name:    "a panic is internal, without its text",
			call:    Call{Method: "v1.panic", RequestID: "client-1"},
			wantRan: 1,
			want:    Reply{Error: &Error{Code: "INTERNAL", Type: "about:blank", Title: "Internal server error", Status: 500, RequestID: "client-1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var ran atomic.Int32
			answer := newDispatcher(t, &ran).Answer(t.Context(), tt.call)
			if ran.Load() != tt.wantRan {
				t.Errorf("handler ran %d times, want %d", ran.Load(), tt.wantRan)
			}
			if strings.Contains(string(answer.Body), "secret") {
				t.Fatalf("reply leaks: %s", answer.Body)
			}
			var got Reply
			if err := json.Unmarshal(answer.Body, &got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("reply (-want +got):\n%s", diff)
			}
			if (answer.Error != nil) != (tt.want.Error != nil) {
				t.Errorf("Answer.Error = %v", answer.Error)
			}
		})
	}
}

func TestDispatcherRequestID(t *testing.T) {
	t.Parallel()
	var ran atomic.Int32
	d := newDispatcher(t, &ran)
	for _, sent := range []string{"", "has space"} {
		answer := d.Answer(t.Context(), Call{Method: "v1.echo", RequestID: sent})
		if !requestid.Valid(answer.RequestID) || answer.RequestID == sent {
			t.Errorf("sent %q, got request ID %q", sent, answer.RequestID)
		}
	}
}

func TestDecodeReply(t *testing.T) {
	t.Parallel()
	var data struct{ Name string }
	if err := DecodeReply([]byte(`{"ok":true,"data":{"Name":"eu"}}`), &data); err != nil || data.Name != "eu" {
		t.Fatalf("data %+v, err %v", data, err)
	}
	err := DecodeReply([]byte(`{"ok":false,"error":{"code":"NOT_FOUND","type":"/problems/not-found","title":"Not found","status":404}}`), &data)
	var rpcErr *Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "NOT_FOUND" {
		t.Fatalf("err = %v", err)
	}
}

func TestPoolBoundsConcurrency(t *testing.T) {
	t.Parallel()
	pool := NewPool(3)
	var running, peak atomic.Int32
	release := make(chan struct{})
	for range 10 {
		go func() {
			_ = pool.Go(t.Context(), func() {
				n := running.Add(1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				<-release
				running.Add(-1)
			})
		}()
	}
	time.Sleep(100 * time.Millisecond)
	if got := running.Load(); got != 3 {
		t.Errorf("%d running, want 3", got)
	}
	close(release)
	time.Sleep(50 * time.Millisecond)
	if err := pool.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 3 {
		t.Errorf("peak %d, want 3", peak.Load())
	}

	full := NewPool(1)
	block := make(chan struct{})
	_ = full.Go(t.Context(), func() { <-block })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := full.Go(ctx, func() {}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Go on a full pool = %v, want the context's error", err)
	}
	close(block)
}

func TestRouter(t *testing.T) {
	t.Parallel()
	r := NewRouter()
	r.Handle("v1.b", nil)
	r.Handle("v1.a", nil)
	if diff := cmp.Diff([]string{"v1.a", "v1.b"}, r.Methods()); diff != "" {
		t.Error(diff)
	}
	for _, bad := range []string{"v1.a", "createLedger"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Handle(%q) did not panic", bad)
				}
			}()
			r.Handle(bad, nil)
		}()
	}
}
