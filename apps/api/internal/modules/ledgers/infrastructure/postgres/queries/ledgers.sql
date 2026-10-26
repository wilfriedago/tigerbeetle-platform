-- name: CreateLedger :one
INSERT INTO ledgers (organization_id, name, metadata, created_at)
VALUES ($4, $1, $2, $3)
RETURNING name, metadata, created_at, organization_id;

-- name: GetLedger :one
SELECT name, metadata, created_at, organization_id
FROM ledgers
WHERE organization_id = $2 AND name = $1;

-- name: ListLedgers :many
SELECT name, metadata, created_at, organization_id
FROM ledgers
WHERE organization_id = sqlc.arg(organization_id) AND name > sqlc.arg(after)::text
    -- Only the ledgers the caller may read (ADR-0027): all of them, or the named ones.
    AND (sqlc.arg(all_ledgers)::boolean OR name = ANY(sqlc.arg(names)::text[]))
ORDER BY name
LIMIT sqlc.arg(page_size);

-- name: ReplaceLedgerMetadata :execrows
UPDATE ledgers
SET metadata = $2
WHERE organization_id = $3 AND name = $1;
