// Package rpc is what the broker RPC transports share (ADR-0021, plan section 11.2): the JSON
// envelope of requests and replies, the method router, the worker pool and the deadline rules.
// internal/platform/amqprpc and natsrpc only move bytes between a broker and Dispatcher.Answer, so
// both brokers answer every method the same way.
package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// Request is one call: its method and the REST parameters and body it carries.
type Request struct {
	// Method is "v1.<operationId>", such as "v1.createTransaction".
	Method string
	// Params holds the path and query parameters under their REST names.
	Params json.RawMessage
	// Body is the REST request body.
	Body json.RawMessage
	// IdempotencyKey is the key of a money write, carried by a message header.
	IdempotencyKey string
}

// Envelope is the JSON of a request message: {"params": {...}, "body": {...}}.
type Envelope struct {
	Params any `json:"params,omitempty"`
	Body   any `json:"body,omitempty"`
}

type rawEnvelope struct {
	Params json.RawMessage `json:"params"`
	Body   json.RawMessage `json:"body"`
}

// Reply is the JSON of every reply: {"ok": true, "data": ...} or {"ok": false, "error": {...}}.
type Reply struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// Error carries the members of a REST problem under the names GraphQL uses, so one client parser
// reads every transport (plan section 9.2).
type Error struct {
	Code       string   `json:"code"`
	Reason     string   `json:"reason,omitempty"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Status     int      `json:"status"`
	Detail     string   `json:"detail,omitempty"`
	RequestID  string   `json:"requestId,omitempty"`
	RetryAfter int      `json:"retryAfter,omitempty"`
	Posting    *int     `json:"posting,omitempty"`
	Results    []string `json:"results,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("rpc: %s %s", e.Code, e.Type)
}

// ErrorOf renders a problem. An internal problem keeps no detail.
func ErrorOf(p problem.Problem, requestID string) *Error {
	e := &Error{Code: string(p.Kind), Reason: p.Reason, Type: p.Type(), Title: p.Title, Status: p.Status, RequestID: requestID, Results: p.Results}
	if p.Kind != problem.KindInternal {
		e.Detail = p.Detail
	}
	if p.RetryAfter > 0 {
		// Whole seconds, rounded up, as the Retry-After header of REST.
		e.RetryAfter = int((p.RetryAfter + 999_999_999) / 1_000_000_000)
	}
	if p.Posting >= 0 {
		posting := p.Posting
		e.Posting = &posting
	}
	return e
}

// decode parses a request message. An empty message is an empty envelope.
func decode(method string, payload []byte, key string) (Request, error) {
	req := Request{Method: method, IdempotencyKey: key}
	if len(bytes.TrimSpace(payload)) == 0 {
		return req, nil
	}
	var env rawEnvelope
	if err := strictUnmarshal(payload, &env); err != nil {
		return req, invalid("the message is not an RPC envelope: " + jsonDetail(err))
	}
	req.Params, req.Body = env.Params, env.Body
	return req, nil
}

// Params decodes the parameters of a request into P. Unknown members are refused, so a typo in a
// filter is an error instead of a silently ignored filter.
func Params[P any](req Request) (P, error) {
	var p P
	if len(req.Params) == 0 || string(req.Params) == "null" {
		return p, nil
	}
	if err := strictUnmarshal(req.Params, &p); err != nil {
		return p, invalid("params: " + jsonDetail(err))
	}
	return p, nil
}

// Body decodes the body of a request into B, as REST decodes its JSON body.
func Body[B any](req Request) (B, error) {
	var b B
	if len(req.Body) == 0 || string(req.Body) == "null" {
		return b, nil
	}
	if err := strictUnmarshal(req.Body, &b); err != nil {
		return b, invalid("body: " + jsonDetail(err))
	}
	return b, nil
}

func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}

// jsonDetail keeps what a client needs to find its mistake, without echoing the payload.
func jsonDetail(err error) string {
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return fmt.Sprintf("invalid JSON at offset %d", syntax.Offset)
	case errors.As(err, &typeErr):
		return fmt.Sprintf("%s must be %s", typeErr.Field, typeErr.Type)
	default:
		return err.Error()
	}
}

func invalid(detail string) error {
	return ledger.NewProblem(ledger.ErrInvalid, "invalid-request", detail)
}

// Errors a client sees when no reply comes back.
var (
	// ErrTimeout: the deadline passed without a reply. A write may still have run; retry it with
	// the same idempotency key.
	ErrTimeout = errors.New("rpc: no reply before the deadline")
	// ErrNoResponders: no server listens for the method, so nothing ran.
	ErrNoResponders = errors.New("rpc: no server is listening")
)

// DecodeReply reads a reply into data, for clients. A reply with ok false returns its *Error.
func DecodeReply(body []byte, data any) error {
	var r struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error *Error          `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("rpc: decode reply: %w", err)
	}
	if !r.OK {
		if r.Error == nil {
			return errors.New("rpc: a failed reply without an error")
		}
		return r.Error
	}
	if data == nil || len(r.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.Data, data); err != nil {
		return fmt.Errorf("rpc: decode reply data: %w", err)
	}
	return nil
}
