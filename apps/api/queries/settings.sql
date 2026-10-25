-- name: ListRules :many
SELECT pattern, balance_limit, history, code, created_at, updated_at
FROM account_rules
WHERE ledger = $1;

-- name: PutRule :one
INSERT INTO account_rules (ledger, pattern, balance_limit, history, code, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (ledger, pattern) DO UPDATE
SET balance_limit = EXCLUDED.balance_limit,
    history = EXCLUDED.history,
    code = EXCLUDED.code,
    updated_at = EXCLUDED.updated_at
RETURNING pattern, balance_limit, history, code, created_at, updated_at;

-- name: DeleteRule :execrows
DELETE FROM account_rules
WHERE ledger = $1 AND pattern = $2;

-- name: ListEventTypes :many
SELECT name, code, created_at
FROM event_types
WHERE ledger = $1
ORDER BY code;

-- name: GetEventType :one
SELECT name, code, created_at
FROM event_types
WHERE ledger = $1 AND name = $2;

-- name: NextEventTypeCode :one
SELECT (COALESCE(max(code), 1) + 1)::integer AS code
FROM event_types
WHERE ledger = $1;

-- name: CreateEventType :one
INSERT INTO event_types (ledger, name, code)
VALUES ($1, $2, $3)
RETURNING name, code, created_at;

-- name: EventTypeUsage :many
SELECT u.event_type, count(*)::bigint AS uses
FROM (
    SELECT t.event_type FROM transactions t WHERE t.ledger = $1
    UNION ALL
    SELECT h.event_type FROM holds h WHERE h.ledger = $1
) u
GROUP BY u.event_type;
