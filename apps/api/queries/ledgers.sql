-- name: CreateLedger :one
INSERT INTO ledgers (name, metadata, created_at)
VALUES ($1, $2, $3)
RETURNING name, metadata, created_at;

-- name: GetLedger :one
SELECT name, metadata, created_at
FROM ledgers
WHERE name = $1;

-- name: GetLedgerSummary :one
SELECT
    l.name,
    l.metadata,
    l.created_at,
    COALESCE((SELECT array_agg(la.asset ORDER BY la.asset) FROM ledger_assets la WHERE la.ledger = l.name), '{}')::text[] AS assets,
    (SELECT count(*) FROM addresses a WHERE a.ledger = l.name)::bigint AS accounts
FROM ledgers l
WHERE l.name = $1;

-- name: ListLedgers :many
SELECT
    l.name,
    l.metadata,
    l.created_at,
    COALESCE((SELECT array_agg(la.asset ORDER BY la.asset) FROM ledger_assets la WHERE la.ledger = l.name), '{}')::text[] AS assets,
    (SELECT count(*) FROM addresses a WHERE a.ledger = l.name)::bigint AS accounts
FROM ledgers l
WHERE l.name > sqlc.arg(after)::text
ORDER BY l.name
LIMIT sqlc.arg(page_size);

-- name: ReplaceLedgerMetadata :execrows
UPDATE ledgers
SET metadata = $2
WHERE name = $1;
