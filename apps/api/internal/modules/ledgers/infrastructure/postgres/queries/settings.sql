-- name: ListRules :many
SELECT pattern, balance_limit, history, code, created_at, updated_at
FROM account_rules
WHERE organization_id = $2 AND ledger = $1;

-- name: PutRule :one
INSERT INTO account_rules (organization_id, ledger, pattern, balance_limit, history, code, created_at, updated_at)
VALUES ($7, $1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (organization_id, ledger, pattern) DO UPDATE
SET balance_limit = EXCLUDED.balance_limit,
    history = EXCLUDED.history,
    code = EXCLUDED.code,
    updated_at = EXCLUDED.updated_at
RETURNING pattern, balance_limit, history, code, created_at, updated_at;

-- name: DeleteRule :execrows
DELETE FROM account_rules
WHERE organization_id = $3 AND ledger = $1 AND pattern = $2;

-- name: ListEventTypes :many
SELECT name, code, created_at
FROM event_types
WHERE organization_id = $2 AND ledger = $1
ORDER BY code;

-- name: GetEventType :one
SELECT name, code, created_at
FROM event_types
WHERE organization_id = $3 AND ledger = $1 AND name = $2;

-- name: NextEventTypeCode :one
SELECT (COALESCE(max(code), 1) + 1)::integer AS code
FROM event_types
WHERE organization_id = $2 AND ledger = $1;

-- name: CreateEventType :one
INSERT INTO event_types (organization_id, ledger, name, code)
VALUES ($4, $1, $2, $3)
RETURNING name, code, created_at;
