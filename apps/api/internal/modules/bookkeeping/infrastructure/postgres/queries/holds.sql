-- A hold is expired once its timeout has passed on the clock passed as `now`, even while the row
-- still says open: TigerBeetle has released it (ADR-0010).

-- name: InsertHold :exec
INSERT INTO holds (organization_id, id, ledger, pending_transfer_id, source_account, destination_account, reference, event_type, code,
    metadata, timeout_seconds, idempotency_key, request_id, created_at, tb_timestamp, status)
VALUES ($15, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'open')
ON CONFLICT (id) DO NOTHING;

-- name: ResolveHold :execrows
UPDATE holds
SET status = $3, resolution_transfer_id = $4, resolved_at = $5
WHERE organization_id = $6 AND ledger = $1 AND id = $2 AND status IN ('open', 'expired');

-- name: MarkHoldExpired :exec
UPDATE holds
SET status = 'expired'
WHERE organization_id = $3 AND ledger = $1 AND id = $2 AND status = 'open';

-- name: GetHold :one
SELECT h.id, h.ledger, h.pending_transfer_id, h.reference, h.event_type, h.code, h.metadata, h.timeout_seconds,
    h.idempotency_key, h.request_id, h.created_at, h.tb_timestamp, h.status, h.resolution_transfer_id, h.resolved_at,
    sqlc.embed(s), sqlc.embed(d)
FROM holds h
JOIN accounts s ON s.id = h.source_account
JOIN accounts d ON d.id = h.destination_account
WHERE h.organization_id = $3 AND h.ledger = $1 AND h.id = $2;

-- name: ListHolds :many
SELECT h.id, h.ledger, h.pending_transfer_id, h.reference, h.event_type, h.code, h.metadata, h.timeout_seconds,
    h.idempotency_key, h.request_id, h.created_at, h.tb_timestamp, h.status, h.resolution_transfer_id, h.resolved_at,
    sqlc.embed(s), sqlc.embed(d)
FROM holds h
JOIN accounts s ON s.id = h.source_account
JOIN accounts d ON d.id = h.destination_account
WHERE h.organization_id = sqlc.arg(organization_id) AND h.ledger = sqlc.arg(ledger)
    AND (sqlc.narg(before_id)::uuid IS NULL OR h.id < sqlc.narg(before_id)::uuid)
    AND (
        sqlc.narg(status)::text IS NULL
        OR (sqlc.narg(status)::text = 'open' AND h.status = 'open'
            AND (h.timeout_seconds = 0 OR h.created_at + make_interval(secs => h.timeout_seconds) > sqlc.arg(now)::timestamptz))
        OR (sqlc.narg(status)::text = 'expired' AND (h.status = 'expired' OR (h.status = 'open'
            AND h.timeout_seconds > 0 AND h.created_at + make_interval(secs => h.timeout_seconds) <= sqlc.arg(now)::timestamptz)))
        OR (sqlc.narg(status)::text IN ('captured', 'voided') AND h.status = sqlc.narg(status)::text)
    )
    AND (
        sqlc.narg(prefix)::text IS NULL
        OR s.address = sqlc.narg(prefix)::text
        OR (s.address >= sqlc.narg(prefix)::text || ':' AND s.address < sqlc.narg(prefix)::text || ';')
        OR d.address = sqlc.narg(prefix)::text
        OR (d.address >= sqlc.narg(prefix)::text || ':' AND d.address < sqlc.narg(prefix)::text || ';')
    )
ORDER BY h.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountHolds :one
SELECT
    count(*) FILTER (WHERE status = 'open' AND (timeout_seconds = 0 OR created_at + make_interval(secs => timeout_seconds) > sqlc.arg(now)::timestamptz))::bigint AS open,
    count(*) FILTER (WHERE status = 'captured')::bigint AS captured,
    count(*) FILTER (WHERE status = 'voided')::bigint AS voided,
    count(*) FILTER (WHERE status = 'expired' OR (status = 'open' AND timeout_seconds > 0
        AND created_at + make_interval(secs => timeout_seconds) <= sqlc.arg(now)::timestamptz))::bigint AS expired
FROM holds
WHERE organization_id = sqlc.arg(organization_id) AND ledger = sqlc.arg(ledger);

-- name: OpenHolds :many
SELECT h.id, h.ledger, h.pending_transfer_id, h.reference, h.event_type, h.code, h.metadata, h.timeout_seconds,
    h.idempotency_key, h.request_id, h.created_at, h.tb_timestamp, h.status, h.resolution_transfer_id, h.resolved_at,
    sqlc.embed(s), sqlc.embed(d)
FROM holds h
JOIN accounts s ON s.id = h.source_account
JOIN accounts d ON d.id = h.destination_account
WHERE h.organization_id = sqlc.arg(organization_id) AND h.ledger = sqlc.arg(ledger) AND h.status = 'open'
    AND (h.timeout_seconds = 0 OR h.created_at + make_interval(secs => h.timeout_seconds) > sqlc.arg(now)::timestamptz)
ORDER BY h.id DESC
LIMIT sqlc.arg(max_rows);

-- name: CountOpenHoldsFrom :one
SELECT count(*)::bigint
FROM holds h
JOIN accounts s ON s.id = h.source_account
WHERE h.organization_id = sqlc.arg(organization_id) AND h.ledger = sqlc.arg(ledger) AND s.address = sqlc.arg(address)::text AND h.status = 'open'
    AND (h.timeout_seconds = 0 OR h.created_at + make_interval(secs => h.timeout_seconds) > sqlc.arg(now)::timestamptz);
