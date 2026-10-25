//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// graphql posts one operation and decodes the answer.
func (h *harness) graphql(query string, variables map[string]any) map[string]any {
	h.t.Helper()
	raw, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		h.t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.server.URL+"/graphql", bytes.NewReader(raw))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		h.t.Fatalf("status %d, body %q: %v", res.StatusCode, body, err)
	}
	return out
}

// TestGraphQLEndToEnd runs GraphQL through bootstrap.Build against real Postgres and TigerBeetle:
// the same use cases as REST, the same idempotency records, the same problems.
func TestGraphQLEndToEnd(t *testing.T) {
	h := newHarness(t)

	created := h.graphql(`mutation { createLedger(input: {name: "gql", metadata: {team: "payments"}}) { name accountCount } }`, nil)
	if created["errors"] != nil || dig(created, "data", "createLedger", "name") != "gql" {
		t.Fatalf("createLedger = %v", created)
	}

	const create = `mutation($input: CreateTransactionInput!) { createTransaction(input: $input) { id postings { amount transferId } } }`
	posting := map[string]any{"source": "world", "destination": "users:1:wallet", "asset": "EUR/2", "amount": "1250"}
	input := map[string]any{"ledger": "gql", "idempotencyKey": "cross-1", "postings": []any{posting}}
	first := h.graphql(create, map[string]any{"input": input})
	if first["errors"] != nil {
		t.Fatalf("createTransaction = %v", first)
	}
	id := dig(first, "data", "createTransaction", "id")

	// The same write retried over REST with the same key replays: the body hash covers the
	// normalised command, not the wire bytes (plan section 10).
	replay := h.do(http.MethodPost, "/v1/ledgers/gql/transactions", "cross-1", map[string]any{"postings": []any{posting}})
	h.expect(replay, http.StatusCreated, "REST replay of a GraphQL write")
	if replay.str("id") != id {
		t.Fatalf("REST replay answered %s, the GraphQL write created %v", replay.str("id"), id)
	}
	reused := h.do(http.MethodPost, "/v1/ledgers/gql/transactions", "cross-1",
		map[string]any{"postings": []any{map[string]any{"source": "world", "destination": "users:1:wallet", "asset": "EUR/2", "amount": "1"}}})
	h.expect(reused, http.StatusUnprocessableEntity, "a key reused with a different body")

	read := h.graphql(`query { ledger(name: "gql") {
		accountCount assetCodes metadata
		account(address: "users:1:wallet") { balances { asset balance } }
		transactions(first: 10) { nodes { id } pageInfo { hasNextPage } }
		requests { nodes { key status } }
		missingHold: hold(id: "00000000000000000000000000000001") { id }
	} nope: ledger(name: "nope") { name } }`, nil)
	if read["errors"] != nil {
		t.Fatalf("read = %v", read)
	}
	if dig(read, "data", "ledger", "accountCount") != float64(2) ||
		dig(read, "data", "ledger", "account", "balances", "0", "balance") != "1250" ||
		dig(read, "data", "ledger", "transactions", "nodes", "0", "id") != id ||
		dig(read, "data", "ledger", "requests", "nodes", "0", "status") != "committed" ||
		dig(read, "data", "ledger", "missingHold") != nil || dig(read, "data", "nope") != nil {
		t.Fatalf("read = %v", read["data"])
	}

	declined := h.graphql(create, map[string]any{"input": map[string]any{
		"ledger": "gql", "idempotencyKey": "declined-1", "postings": []any{
			map[string]any{"source": "world", "destination": "users:2:wallet", "asset": "EUR/2", "amount": "5"},
			map[string]any{"source": "users:2:wallet", "destination": "merchants:1:main", "asset": "EUR/2", "amount": "999999"},
		},
	}})
	if dig(declined, "errors", "0", "extensions", "code") != "REJECTED" ||
		dig(declined, "errors", "0", "extensions", "reason") != "exceeds-credits" ||
		dig(declined, "errors", "0", "extensions", "posting") != float64(1) {
		t.Fatalf("declined = %v", declined)
	}
}

// dig walks decoded JSON by map keys and slice indexes.
func dig(v any, path ...string) any {
	for _, p := range path {
		switch m := v.(type) {
		case map[string]any:
			v = m[p]
		case []any:
			var i int
			for _, c := range p {
				i = i*10 + int(c-'0')
			}
			if i >= len(m) {
				return nil
			}
			v = m[i]
		default:
			return nil
		}
	}
	return v
}
