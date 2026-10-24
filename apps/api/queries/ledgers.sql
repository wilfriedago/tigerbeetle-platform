-- name: CreateLedger :one
INSERT INTO ledgers (name, metadata, created_at)
VALUES ($1, $2, $3)
RETURNING name, metadata, created_at;

-- name: GetLedger :one
SELECT name, metadata, created_at
FROM ledgers
WHERE name = $1;
