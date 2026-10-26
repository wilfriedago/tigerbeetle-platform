-- name: InsertTransaction :exec
INSERT INTO transactions (organization_id, id, ledger, reference, event_type, code, business_time, metadata, idempotency_key, request_id, reverts, committed_at, tb_timestamp)
VALUES ($13, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (id) DO NOTHING;

-- name: InsertPostings :exec
INSERT INTO transaction_postings (transaction_id, position, transfer_id, source_account, destination_account)
SELECT
    sqlc.arg(transaction_id)::uuid,
    unnest(sqlc.arg(positions)::smallint[]),
    unnest(sqlc.arg(transfer_ids)::uuid[]),
    unnest(sqlc.arg(sources)::uuid[]),
    unnest(sqlc.arg(destinations)::uuid[])
ON CONFLICT DO NOTHING;

-- name: GetTransaction :one
SELECT t.id, t.ledger, t.reference, t.event_type, t.code, t.business_time, t.metadata, t.idempotency_key,
    t.request_id, t.reverts, t.committed_at, t.tb_timestamp,
    (SELECT r.id FROM transactions r WHERE r.reverts = t.id) AS reverted_by
FROM transactions t
WHERE t.organization_id = $3 AND t.ledger = $1 AND t.id = $2;

-- name: ListTransactions :many
SELECT t.id, t.ledger, t.reference, t.event_type, t.code, t.business_time, t.metadata, t.idempotency_key,
    t.request_id, t.reverts, t.committed_at, t.tb_timestamp,
    (SELECT r.id FROM transactions r WHERE r.reverts = t.id) AS reverted_by
FROM transactions t
WHERE t.organization_id = sqlc.arg(organization_id) AND t.ledger = sqlc.arg(ledger)
    AND (sqlc.narg(before_id)::uuid IS NULL OR t.id < sqlc.narg(before_id)::uuid)
    AND (sqlc.narg(reference)::text IS NULL OR t.reference = sqlc.narg(reference)::text)
    AND (sqlc.narg(event_type)::text IS NULL OR t.event_type = sqlc.narg(event_type)::text)
    AND (sqlc.narg(committed_after)::timestamptz IS NULL OR t.committed_at >= sqlc.narg(committed_after)::timestamptz)
    AND (sqlc.narg(committed_before)::timestamptz IS NULL OR t.committed_at < sqlc.narg(committed_before)::timestamptz)
    AND (
        (sqlc.narg(prefix)::text IS NULL AND sqlc.narg(asset)::text IS NULL)
        OR EXISTS (
            SELECT 1
            FROM transaction_postings p
            JOIN accounts a ON a.id IN (p.source_account, p.destination_account)
            WHERE p.transaction_id = t.id
                AND (sqlc.narg(asset)::text IS NULL OR a.asset = sqlc.narg(asset)::text)
                AND (
                    sqlc.narg(prefix)::text IS NULL
                    OR a.address = sqlc.narg(prefix)::text
                    OR (a.address >= sqlc.narg(prefix)::text || ':' AND a.address < sqlc.narg(prefix)::text || ';')
                )
        )
    )
ORDER BY t.id DESC
LIMIT sqlc.arg(page_size);

-- name: TransactionPostings :many
-- tenant-free: by transaction IDs the caller read inside its organization.
SELECT p.transaction_id, p.position, p.transfer_id, sqlc.embed(s), sqlc.embed(d)
FROM transaction_postings p
JOIN accounts s ON s.id = p.source_account
JOIN accounts d ON d.id = p.destination_account
WHERE p.transaction_id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY p.transaction_id, p.position;

-- name: ReplaceTransactionMetadata :execrows
UPDATE transactions
SET metadata = $3
WHERE organization_id = $4 AND ledger = $1 AND id = $2;

-- name: SearchTransactions :many
SELECT t.id, t.ledger, t.reference, t.event_type, t.code, t.business_time, t.metadata, t.idempotency_key,
    t.request_id, t.reverts, t.committed_at, t.tb_timestamp,
    (SELECT r.id FROM transactions r WHERE r.reverts = t.id) AS reverted_by
FROM transactions t
WHERE t.organization_id = sqlc.arg(organization_id) AND t.ledger = sqlc.arg(ledger)
    AND (t.id = sqlc.narg(id)::uuid OR (t.reference IS NOT NULL AND starts_with(t.reference, sqlc.arg(text)::text)))
ORDER BY t.id DESC
LIMIT sqlc.arg(max_rows);
