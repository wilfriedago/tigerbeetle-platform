#!/usr/bin/env bash
# Seeds a demo ledger through the public API, so the console has something to show: assets in two
# scales, a rule with balance history, event types, deposits, payments with fees, a revert, a
# declined write, and holds that are open, captured and voided.
# Usage: scripts/seed-demo.sh [ledger]   (API_URL defaults to http://localhost:8080)
# Running it twice adds more activity to the same ledger; nothing is ever deleted.
set -euo pipefail

api="${API_URL:-http://localhost:8080}"
ledger="${1:-eu-payments}"
base="$api/v1/ledgers/$ledger"

key() { echo "idem_seed-$(uuidgen | tr '[:upper:]' '[:lower:]')"; }

# call METHOD URL [BODY] [IDEMPOTENCY_KEY] prints the response body; a non-2xx answer is printed
# to stderr and does not stop the script, because a declined write is part of the demo.
call() {
  local method="$1" url="$2" body="${3:-}" idem="${4:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json' -w '\n%{http_code}')
  [[ -n "$idem" ]] && args+=(-H "Idempotency-Key: $idem")
  [[ -n "$body" ]] && args+=(-d "$body")
  local out code
  out="$(curl "${args[@]}" "$url")"
  code="${out##*$'\n'}"
  out="${out%$'\n'*}"
  if [[ "$code" -ge 300 ]]; then
    echo "    $method ${url#"$api"} -> $code $(echo "$out" | jq -r '.title // empty' 2>/dev/null)" >&2
  fi
  echo "$out"
}

tx() { call POST "$base/transactions" "$1" "$(key)" >/dev/null; }

echo "==> ledger $ledger"
call POST "$base" '{"metadata":{"region":"eu-west","owner":"payments-team"}}' >/dev/null

echo "==> rules and event types"
call PUT "$base/rules/users:" '{"limit":"debits_must_not_exceed_credits","history":true,"code":10}' >/dev/null
call PUT "$base/rules/merchants:" '{"limit":"debits_must_not_exceed_credits","history":false,"code":20}' >/dev/null
for name in deposit payment fee payout transfer card.authorized; do
  call POST "$base/event-types" "{\"name\":\"$name\"}" >/dev/null
done

echo "==> deposits"
tx '{"postings":[{"source":"world","destination":"users:1234:wallet","asset":"EUR/2","amount":"500000"}],"reference":"dep-2026-0001","eventType":"deposit","metadata":{"channel":"bank_transfer"}}'
tx '{"postings":[{"source":"world","destination":"users:5678:wallet","asset":"EUR/2","amount":"250000"}],"reference":"dep-2026-0002","eventType":"deposit"}'
tx '{"postings":[{"source":"world","destination":"users:1618:wallet","asset":"EUR/2","amount":"90000"}],"reference":"dep-2026-0003","eventType":"deposit"}'
tx '{"postings":[{"source":"world","destination":"users:2718:savings","asset":"EUR/2","amount":"300000"}],"reference":"dep-2026-0004","eventType":"deposit"}'
tx '{"postings":[{"source":"world","destination":"users:3141:wallet","asset":"XOF/0","amount":"250000"}],"reference":"dep-xof-0091","eventType":"deposit"}'
tx '{"postings":[{"source":"world","destination":"treasury:fx:eur","asset":"EUR/2","amount":"10000000"}],"reference":"treasury-funding","eventType":"deposit"}'

echo "==> payments with fees"
tx '{"postings":[{"source":"users:1234:wallet","destination":"merchants:42:main","asset":"EUR/2","amount":"120000"},{"source":"users:1234:wallet","destination":"fees:card","asset":"EUR/2","amount":"5000"}],"reference":"ord_8812","eventType":"payment","metadata":{"order_id":"ord_8812","channel":"card"}}'
tx '{"postings":[{"source":"users:5678:wallet","destination":"merchants:42:main","asset":"EUR/2","amount":"8990"},{"source":"users:5678:wallet","destination":"fees:card","asset":"EUR/2","amount":"120"}],"reference":"ord_8809","eventType":"payment"}'
tx '{"postings":[{"source":"users:1618:wallet","destination":"merchants:77:main","asset":"EUR/2","amount":"4000"}],"reference":"ord_8790","eventType":"payment"}'
tx '{"postings":[{"source":"treasury:fx:eur","destination":"treasury:fx:pool","asset":"EUR/2","amount":"500000"}],"reference":"fx-20261024-07","eventType":"transfer"}'

echo "==> a revert"
sweep="$(call POST "$base/transactions" '{"postings":[{"source":"users:2718:savings","destination":"users:2718:wallet","asset":"EUR/2","amount":"30000"}],"reference":"sweep-3301","eventType":"transfer"}' "$(key)")"
sweep_id="$(echo "$sweep" | jq -r '.id')"
call POST "$base/transactions/$sweep_id/revert" '{"reference":"sweep-3301-r"}' "$(key)" >/dev/null

echo "==> a payout to world and a declined payment"
tx '{"postings":[{"source":"merchants:42:main","destination":"world","asset":"EUR/2","amount":"100000"}],"reference":"payout-1188","eventType":"payout"}'
tx '{"postings":[{"source":"users:1618:wallet","destination":"merchants:42:main","asset":"EUR/2","amount":"990000"}],"reference":"ord_8820","eventType":"payment"}' || true

echo "==> holds"
hold() { call POST "$base/holds" "$1" "$(key)"; }
hold '{"source":"users:1234:wallet","destination":"merchants:42:main","asset":"EUR/2","amount":"200000","timeoutSeconds":2400,"reference":"auth-5521","eventType":"card.authorized"}' >/dev/null
hold '{"source":"users:5678:wallet","destination":"merchants:77:main","asset":"EUR/2","amount":"4000","timeoutSeconds":720,"reference":"auth-5522","eventType":"card.authorized"}' >/dev/null
captured="$(hold '{"source":"users:5678:wallet","destination":"merchants:42:main","asset":"EUR/2","amount":"15000","timeoutSeconds":3600,"reference":"auth-5523","eventType":"card.authorized"}' | jq -r '.id')"
call POST "$base/holds/$captured/capture" '{"amount":"12000"}' "$(key)" >/dev/null
voided="$(hold '{"source":"users:1618:wallet","destination":"merchants:77:main","asset":"EUR/2","amount":"2500","reference":"auth-5524","eventType":"card.authorized"}' | jq -r '.id')"
call POST "$base/holds/$voided/void" '' "$(key)" >/dev/null

echo "==> done: $api, ledger $ledger"
