-- Owned by the bookkeeping module (ADR-0031).
-- +goose Up
-- The writes of one principal on a ledger, newest first: the recent writes of an API key or a
-- machine client (ADR-0029).
CREATE INDEX write_requests_actor_created ON write_requests (organization_id, ledger, actor_subject, created_at DESC, idempotency_key)
    WHERE actor_subject IS NOT NULL;

-- +goose Down
DROP INDEX write_requests_actor_created;
