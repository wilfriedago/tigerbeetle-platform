-- name: InsertWrite :one
INSERT INTO write_requests (organization_id, ledger, idempotency_key, operation, body_hash, request, target_id, transfer_ids, request_id, created_at, actor_issuer, actor_subject)
VALUES ($10, $1, $2, $3, $4, $5, $6, $7, $8, $9, $11, $12)
ON CONFLICT (organization_id, ledger, idempotency_key) DO NOTHING
RETURNING ledger, idempotency_key, operation, status, body_hash, request, target_id, transfer_ids,
    response_status, response, problem_type, request_id, attempts, created_at, resolved_at, organization_id, actor_issuer, actor_subject;

-- name: GetWrite :one
SELECT ledger, idempotency_key, operation, status, body_hash, request, target_id, transfer_ids,
    response_status, response, problem_type, request_id, attempts, created_at, resolved_at, organization_id, actor_issuer, actor_subject
FROM write_requests
WHERE organization_id = $3 AND ledger = $1 AND idempotency_key = $2;

-- name: ListWrites :many
SELECT ledger, idempotency_key, operation, status, body_hash, request, target_id, transfer_ids,
    response_status, response, problem_type, request_id, attempts, created_at, resolved_at, organization_id, actor_issuer, actor_subject
FROM write_requests
WHERE organization_id = sqlc.arg(organization_id) AND ledger = sqlc.arg(ledger)
    AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
    AND (sqlc.narg(operation)::text IS NULL OR operation = sqlc.narg(operation)::text)
    AND (
        sqlc.narg(search)::text IS NULL
        OR starts_with(idempotency_key, sqlc.narg(search)::text)
        OR request_id = sqlc.narg(search)::text
    )
    AND (
        sqlc.narg(cursor_at)::timestamptz IS NULL
        OR (created_at, idempotency_key) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_key)::text)
    )
ORDER BY created_at DESC, idempotency_key DESC
LIMIT sqlc.arg(page_size);

-- name: NoteAttempt :exec
UPDATE write_requests
SET attempts = attempts + 1
WHERE organization_id = $3 AND ledger = $1 AND idempotency_key = $2;

-- name: ResolveWrite :execrows
UPDATE write_requests
SET status = $3, response_status = $4, response = $5, problem_type = $6, resolved_at = $7
WHERE organization_id = $8 AND ledger = $1 AND idempotency_key = $2 AND status = 'pending';

-- name: PendingWrites :many
-- tenant-free: the reconciler settles every organization's writes; each record carries its
-- organization, which the reconciler acts for.
SELECT ledger, idempotency_key, operation, status, body_hash, request, target_id, transfer_ids,
    response_status, response, problem_type, request_id, attempts, created_at, resolved_at, organization_id, actor_issuer, actor_subject
FROM write_requests
WHERE status = 'pending' AND created_at < sqlc.arg(before)
ORDER BY created_at
LIMIT sqlc.arg(max_rows);

-- name: CountPending :one
-- The reconciler's backlog in one organization, for its status page.
SELECT count(*)::bigint AS pending, count(*) FILTER (WHERE created_at < sqlc.arg(before))::bigint AS stuck
FROM write_requests
WHERE organization_id = sqlc.arg(organization_id) AND status = 'pending';

-- name: DeclinedSince :many
SELECT COALESCE(problem_type, 'unknown')::text AS reason, count(*)::bigint AS occurrences
FROM write_requests
WHERE organization_id = sqlc.arg(organization_id) AND ledger = $1 AND status IN ('declined', 'failed') AND resolved_at >= sqlc.arg(since)
GROUP BY 1
ORDER BY 2 DESC, 1;
