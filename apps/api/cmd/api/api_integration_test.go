//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
)

// faultyLedger wraps the TigerBeetle port to simulate the two crash windows of ADR-0007: a
// request lost before it reaches the cluster, and an answer lost after the chain landed.
type faultyLedger struct {
	app.LedgerStore
	dropBefore atomic.Bool
	dropAfter  atomic.Bool
}

func (f *faultyLedger) CreateTransfers(ctx context.Context, transfers []app.TransferSpec) ([]app.TransferOutcome, error) {
	if f.dropBefore.CompareAndSwap(true, false) {
		return nil, fmt.Errorf("simulated: %w", ledger.ErrUnavailable)
	}
	out, err := f.LedgerStore.CreateTransfers(ctx, transfers)
	if err == nil && f.dropAfter.CompareAndSwap(true, false) {
		return nil, fmt.Errorf("simulated lost answer: %w", ledger.ErrUnavailable)
	}
	return out, err
}

type harness struct {
	t      *testing.T
	server *httptest.Server
	svc    *service
	faults *faultyLedger
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cfg := config.Config{
		PostgresURL:           devstack.StartPostgres(t),
		TigerBeetleAddresses:  []string{devstack.StartTigerBeetle(t)},
		MigrateOnStart:        true,
		ReadinessTimeout:      2 * time.Second,
		TigerBeetleTimeout:    10 * time.Second,
		ReconcilerGracePeriod: time.Millisecond,
		ReconcilerInterval:    time.Hour,
	}
	h := &harness{t: t}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := wire(context.Background(), cfg, logger, func(w *wiring) {
		w.wrapLedgerStore = func(s app.LedgerStore) app.LedgerStore {
			h.faults = &faultyLedger{LedgerStore: s}
			return h.faults
		}
	})
	if err != nil {
		t.Fatalf("wire: %v", err)
	}
	t.Cleanup(svc.close)
	h.svc = svc
	h.server = httptest.NewServer(svc.handler)
	t.Cleanup(h.server.Close)
	return h
}

type response struct {
	status int
	header http.Header
	body   map[string]any
}

func (r response) str(path ...string) string {
	var v any = r.body
	for _, p := range path {
		switch m := v.(type) {
		case map[string]any:
			v = m[p]
		case []any:
			var i int
			_, _ = fmt.Sscanf(p, "%d", &i)
			if i < len(m) {
				v = m[i]
			} else {
				v = nil
			}
		}
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func (h *harness) do(method, path, key string, body any) response {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out := response{status: res.StatusCode, header: res.Header, body: map[string]any{}}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (h *harness) expect(r response, status int, what string) {
	h.t.Helper()
	if r.status != status {
		h.t.Fatalf("%s: status %d, want %d; body %v", what, r.status, status, r.body)
	}
}

// balance returns the balance and available amount of an address in an asset.
func (h *harness) balance(address, asset string) (balance, available, reserved string) {
	h.t.Helper()
	r := h.do(http.MethodGet, "/v1/ledgers/"+testLedger+"/accounts/"+address, "", nil)
	h.expect(r, 200, "get account "+address)
	for _, b := range r.body["balances"].([]any) {
		m := b.(map[string]any)
		if m["asset"] == asset {
			return m["balance"].(string), m["available"].(string), m["reserved"].(string)
		}
	}
	h.t.Fatalf("%s has no %s balance", address, asset)
	return "", "", ""
}

// testLedger is the ledger the end-to-end test works in.
const testLedger = "eu-payments"

func posting(src, dst, asset, amount string) map[string]string {
	return map[string]string{"source": src, "destination": dst, "asset": asset, "amount": amount}
}

func TestLedgerAPIEndToEnd(t *testing.T) {
	h := newHarness(t)
	base := "/v1/ledgers/" + testLedger

	t.Run("ledgers", func(t *testing.T) {
		h.expect(h.do(http.MethodPost, base, "", map[string]any{"metadata": map[string]string{"region": "eu-west"}}), 201, "create ledger")
		h.expect(h.do(http.MethodPost, base, "", map[string]any{}), 409, "duplicate ledger")
		r := h.do(http.MethodPost, "/v1/ledgers/EU", "", map[string]any{})
		h.expect(r, 400, "invalid name")
		if !strings.Contains(r.str("detail"), "ledger name") {
			t.Fatalf("validation detail = %q", r.str("detail"))
		}
		h.expect(h.do(http.MethodGet, "/v1/ledgers/missing", "", nil), 404, "missing ledger")
	})

	t.Run("rules and event types", func(t *testing.T) {
		h.expect(h.do(http.MethodPut, base+"/rules/users:", "", map[string]any{"limit": "debits_must_not_exceed_credits", "history": true, "code": 10}), 200, "put rule")
		h.expect(h.do(http.MethodPut, base+"/rules/users::", "", map[string]any{"limit": "none", "code": 1}), 400, "bad pattern")
		r := h.do(http.MethodGet, base+"/rules/match?address=users:1:wallet", "", nil)
		h.expect(r, 200, "match rule")
		if r.str("rule", "pattern") != "users:" {
			t.Fatalf("winning rule = %s", r.str("rule", "pattern"))
		}
		r = h.do(http.MethodPost, base+"/event-types", "", map[string]any{"name": "deposit"})
		h.expect(r, 201, "create event type")
		if r.str("code") != "2" {
			t.Fatalf("first event type code = %s, want 2", r.str("code"))
		}
		h.expect(h.do(http.MethodPost, base+"/event-types", "", map[string]any{"name": "deposit"}), 409, "duplicate event type")
	})

	t.Run("transactions are idempotent", func(t *testing.T) {
		body := map[string]any{"postings": []any{posting("world", "users:1:wallet", "EUR/2", "100000")}, "eventType": "deposit", "reference": "inv-1"}
		first := h.do(http.MethodPost, base+"/transactions", "fund-1", body)
		h.expect(first, 201, "fund")
		replay := h.do(http.MethodPost, base+"/transactions", "fund-1", body)
		h.expect(replay, 201, "replay")
		if first.str("id") != replay.str("id") || first.str("committedAt") != replay.str("committedAt") {
			t.Fatalf("replay answered a different transaction: %v vs %v", first.body, replay.body)
		}
		reused := h.do(http.MethodPost, base+"/transactions", "fund-1", map[string]any{"postings": []any{posting("world", "users:1:wallet", "EUR/2", "1")}})
		h.expect(reused, 422, "key reused")
		if reused.str("type") != "/problems/idempotency-key-reused" {
			t.Fatalf("type = %s", reused.str("type"))
		}
		missing := h.do(http.MethodPost, base+"/transactions", "", body)
		h.expect(missing, 400, "missing key")
		if missing.str("type") != "/problems/missing-idempotency-key" {
			t.Fatalf("type = %s", missing.str("type"))
		}
		if b, _, _ := h.balance("users:1:wallet", "EUR/2"); b != "100000" {
			t.Fatalf("balance after one fund and a replay = %s, want 100000", b)
		}
	})

	t.Run("a declined chain applies nothing", func(t *testing.T) {
		body := map[string]any{"postings": []any{
			posting("world", "users:1:wallet", "EUR/2", "5"),
			posting("users:1:wallet", "merchants:42:main", "EUR/2", "999999"),
		}}
		r := h.do(http.MethodPost, base+"/transactions", "declined-1", body)
		h.expect(r, 422, "declined")
		if r.str("type") != "/problems/exceeds-credits" || r.str("posting") != "1" || r.str("results", "0") != "linked_event_failed" {
			t.Fatalf("decline problem = %v", r.body)
		}
		if b, _, _ := h.balance("users:1:wallet", "EUR/2"); b != "100000" {
			t.Fatalf("balance after a declined chain = %s, want 100000", b)
		}
		h.expect(h.do(http.MethodPost, base+"/transactions", "declined-1", body), 422, "a declined key stays declined")
		rec := h.do(http.MethodGet, base+"/requests/declined-1", "", nil)
		if rec.str("status") != "declined" || rec.str("problemType") != "/problems/exceeds-credits" {
			t.Fatalf("record = %v", rec.body)
		}
	})

	t.Run("a linked chain across assets", func(t *testing.T) {
		r := h.do(http.MethodPost, base+"/transactions", "chain-1", map[string]any{"postings": []any{
			posting("users:1:wallet", "merchants:42:main", "EUR/2", "30000"),
			posting("users:1:wallet", "fees:card", "EUR/2", "500"),
			posting("world", "users:2:wallet", "XOF/0", "250000"),
		}})
		h.expect(r, 201, "chain")
		if r.str("postings", "2", "asset") != "XOF/0" || r.str("postings", "0", "tbLedger") == r.str("postings", "2", "tbLedger") {
			t.Fatalf("each asset needs its own TigerBeetle ledger number: %v", r.body["postings"])
		}
		if b, _, _ := h.balance("users:1:wallet", "EUR/2"); b != "69500" {
			t.Fatalf("balance = %s, want 69500", b)
		}
		if b, _, _ := h.balance("users:2:wallet", "XOF/0"); b != "250000" {
			t.Fatalf("XOF balance = %s, want 250000", b)
		}
	})

	t.Run("revert", func(t *testing.T) {
		funded := h.do(http.MethodPost, base+"/transactions", "fund-3", map[string]any{"postings": []any{posting("world", "users:3:wallet", "EUR/2", "4000")}})
		h.expect(funded, 201, "fund 3")
		id := funded.str("id")
		reverted := h.do(http.MethodPost, base+"/transactions/"+id+"/revert", "revert-3", map[string]any{})
		h.expect(reverted, 201, "revert")
		if reverted.str("reverts") != id || reverted.str("postings", "0", "source") != "users:3:wallet" {
			t.Fatalf("revert = %v", reverted.body)
		}
		original := h.do(http.MethodGet, base+"/transactions/"+id, "", nil)
		if original.str("revertedBy") != reverted.str("id") {
			t.Fatalf("revertedBy = %q, want %q", original.str("revertedBy"), reverted.str("id"))
		}
		again := h.do(http.MethodPost, base+"/transactions/"+id+"/revert", "revert-3b", map[string]any{})
		h.expect(again, 409, "second revert")
		h.expect(h.do(http.MethodPost, base+"/transactions/"+id+"/revert", "revert-3", map[string]any{}), 201, "revert replay")
		if b, _, _ := h.balance("users:3:wallet", "EUR/2"); b != "0" {
			t.Fatalf("balance after revert = %s, want 0", b)
		}
	})

	t.Run("holds", func(t *testing.T) {
		hold := h.do(http.MethodPost, base+"/holds", "hold-1", map[string]any{"source": "users:1:wallet", "destination": "merchants:42:main", "asset": "EUR/2", "amount": "20000", "timeoutSeconds": 3600})
		h.expect(hold, 201, "hold")
		balance, available, reserved := h.balance("users:1:wallet", "EUR/2")
		if balance != "69500" || available != "49500" || reserved != "20000" {
			t.Fatalf("after hold: balance %s available %s reserved %s", balance, available, reserved)
		}
		too := h.do(http.MethodPost, base+"/transactions", "spend-too-much", map[string]any{"postings": []any{posting("users:1:wallet", "merchants:42:main", "EUR/2", "60000")}})
		h.expect(too, 422, "spending reserved funds")
		id := hold.str("id")
		h.expect(h.do(http.MethodPost, base+"/holds/"+id+"/capture", "cap-0", map[string]any{"amount": "0"}), 400, "zero capture")
		captured := h.do(http.MethodPost, base+"/holds/"+id+"/capture", "cap-1", map[string]any{"amount": "15000"})
		h.expect(captured, 200, "partial capture")
		if captured.str("status") != "captured" || captured.str("capturedAmount") != "15000" {
			t.Fatalf("capture = %v", captured.body)
		}
		h.expect(h.do(http.MethodPost, base+"/holds/"+id+"/capture", "cap-2", map[string]any{}), 409, "second capture")
		h.expect(h.do(http.MethodPost, base+"/holds/"+id+"/void", "void-late", nil), 409, "void after capture")
		if b, _, r := h.balance("users:1:wallet", "EUR/2"); b != "54500" || r != "0" {
			t.Fatalf("after capture: balance %s reserved %s", b, r)
		}

		voided := h.do(http.MethodPost, base+"/holds", "hold-2", map[string]any{"source": "users:1:wallet", "destination": "merchants:42:main", "asset": "EUR/2", "amount": "100"})
		h.expect(voided, 201, "hold 2")
		v := h.do(http.MethodPost, base+"/holds/"+voided.str("id")+"/void", "void-2", nil)
		if v.str("status") != "voided" {
			t.Fatalf("void = %v", v.body)
		}

		short := h.do(http.MethodPost, base+"/holds", "hold-3", map[string]any{"source": "users:1:wallet", "destination": "merchants:42:main", "asset": "EUR/2", "amount": "100", "timeoutSeconds": 1})
		h.expect(short, 201, "short hold")
		// Expiry follows the cluster clock. Wait until the API derives it, then give TigerBeetle a
		// moment past the timeout before trying to capture.
		deadline := time.Now().Add(30 * time.Second)
		for h.do(http.MethodGet, base+"/holds/"+short.str("id"), "", nil).str("status") != "expired" {
			if time.Now().After(deadline) {
				t.Fatal("hold never showed as expired")
			}
			time.Sleep(200 * time.Millisecond)
		}
		time.Sleep(time.Second)
		late := h.do(http.MethodPost, base+"/holds/"+short.str("id")+"/capture", "cap-expired", map[string]any{})
		h.expect(late, 409, "capture after expiry")
		if late.str("type") != "/problems/hold-expired" {
			t.Fatalf("type = %s", late.str("type"))
		}
		list := h.do(http.MethodGet, base+"/holds", "", nil)
		counts := list.body["counts"].(map[string]any)
		if counts["captured"] != 1.0 || counts["voided"] != 1.0 || counts["expired"] != 1.0 {
			t.Fatalf("counts = %v", counts)
		}
	})

	t.Run("reads", func(t *testing.T) {
		agg := h.do(http.MethodGet, base+"/aggregate/balances?address=users", "", nil)
		h.expect(agg, 200, "aggregate")
		if agg.str("prefix") != "users:" || agg.str("accounts") != "3" {
			t.Fatalf("aggregate = %v", agg.body)
		}
		seg := h.do(http.MethodGet, base+"/aggregate/segments?prefix=users", "", nil)
		if seg.str("data", "0", "prefix") != "users:1" {
			t.Fatalf("segments = %v", seg.body)
		}
		accounts := h.do(http.MethodGet, base+"/accounts?prefix=users:1", "", nil)
		if len(accounts.body["data"].([]any)) != 1 {
			t.Fatalf("users:1 must not match users:12 or others: %v", accounts.body)
		}
		history := h.do(http.MethodGet, base+"/accounts/users:1:wallet/history?asset=EUR/2", "", nil)
		h.expect(history, 200, "history with the flag")
		h.expect(h.do(http.MethodGet, base+"/accounts/merchants:42:main/history?asset=EUR/2", "", nil), 422, "history without the flag")
		page := h.do(http.MethodGet, base+"/transactions?limit=2", "", nil)
		if len(page.body["data"].([]any)) != 2 || page.str("next") == "" {
			t.Fatalf("first page = %v", page.body)
		}
		next := h.do(http.MethodGet, base+"/transactions?limit=2&cursor="+page.str("next"), "", nil)
		if next.str("data", "0", "id") == page.str("data", "0", "id") {
			t.Fatal("the cursor did not advance")
		}
		byAddress := h.do(http.MethodGet, base+"/transactions?address=users:2", "", nil)
		if len(byAddress.body["data"].([]any)) != 1 {
			t.Fatalf("filter by address = %v", byAddress.body)
		}
		stats := h.do(http.MethodGet, base+"/stats", "", nil)
		h.expect(stats, 200, "stats")
		if stats.str("declined", "total") == "0" {
			t.Fatalf("stats = %v", stats.body)
		}
		search := h.do(http.MethodGet, base+"/search?q=inv-1", "", nil)
		if len(search.body["transactions"].([]any)) != 1 {
			t.Fatalf("search by reference = %v", search.body)
		}
	})

	t.Run("reconciler settles a write that never reached the cluster", func(t *testing.T) {
		h.faults.dropBefore.Store(true)
		body := map[string]any{"postings": []any{posting("world", "users:4:wallet", "EUR/2", "700")}}
		r := h.do(http.MethodPost, base+"/transactions", "lost-request", body)
		h.expect(r, 503, "lost request")
		if r.str("type") != "/problems/write-pending" {
			t.Fatalf("type = %s", r.str("type"))
		}
		inFlight := h.do(http.MethodPost, base+"/transactions", "lost-request", body)
		h.expect(inFlight, 409, "retry while pending")
		if inFlight.header.Get("Retry-After") == "" {
			t.Fatal("409 request-in-flight without Retry-After")
		}
		if n := h.svc.reconciler.RunOnce(context.Background()); n != 1 {
			t.Fatalf("reconciler settled %d records, want 1", n)
		}
		h.expect(h.do(http.MethodPost, base+"/transactions", "lost-request", body), 201, "outcome after settling")
		if b, _, _ := h.balance("users:4:wallet", "EUR/2"); b != "700" {
			t.Fatalf("balance = %s, want 700 (applied once)", b)
		}
	})

	t.Run("reconciler records a chain whose answer was lost", func(t *testing.T) {
		h.faults.dropAfter.Store(true)
		body := map[string]any{"postings": []any{posting("world", "users:5:wallet", "EUR/2", "900"), posting("users:5:wallet", "fees:card", "EUR/2", "100")}}
		h.expect(h.do(http.MethodPost, base+"/transactions", "lost-answer", body), 503, "lost answer")
		if n := h.svc.reconciler.RunOnce(context.Background()); n != 1 {
			t.Fatalf("reconciler settled %d records, want 1", n)
		}
		r := h.do(http.MethodPost, base+"/transactions", "lost-answer", body)
		h.expect(r, 201, "outcome after settling")
		if b, _, _ := h.balance("users:5:wallet", "EUR/2"); b != "800" {
			t.Fatalf("balance = %s, want 800 (applied once, not resubmitted)", b)
		}
	})
}
