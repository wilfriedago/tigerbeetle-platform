-- Owned by the bookkeeping module (ADR-0030).
-- +goose Up
-- The principal that made the write. A key replays only for it; records written before this
-- column existed have no actor and replay to no one.
ALTER TABLE write_requests ADD COLUMN actor_issuer text;
ALTER TABLE write_requests ADD COLUMN actor_subject text;

-- +goose Down
ALTER TABLE write_requests DROP COLUMN actor_subject;
ALTER TABLE write_requests DROP COLUMN actor_issuer;
