-- name: CreateLedger :one
INSERT INTO ledgers (name, metadata, created_at)
VALUES ($1, $2, $3)
RETURNING name, metadata, created_at;

-- name: GetLedger :one
SELECT name, metadata, created_at
FROM ledgers
WHERE name = $1;

-- name: ListLedgers :many
SELECT name, metadata, created_at
FROM ledgers
WHERE name > sqlc.arg(after)::text
ORDER BY name
LIMIT sqlc.arg(page_size);

-- name: ReplaceLedgerMetadata :execrows
UPDATE ledgers
SET metadata = $2
WHERE name = $1;
