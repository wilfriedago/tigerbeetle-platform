-- +goose Up
-- Idempotency records (ADR-0007). A record is inserted pending, with every ID the write will use,
-- before TigerBeetle is called, so a crash leaves evidence of which IDs were in flight and the
-- reconciler can settle it. A record then becomes committed, declined or failed and never moves
-- back. The canonical request is kept so the reconciler can rebuild the chain; it holds context,
-- never a balance (ADR-0002).
CREATE TABLE write_requests (
    ledger text NOT NULL REFERENCES ledgers (name),
    idempotency_key text NOT NULL,
    operation text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    body_hash bytea NOT NULL,
    request jsonb NOT NULL,
    -- The transaction or hold minted for this write.
    target_id uuid NOT NULL,
    -- One minted ID per TigerBeetle transfer, in chain order.
    transfer_ids uuid[] NOT NULL,
    response_status integer,
    response jsonb,
    problem_type text,
    request_id text,
    attempts integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    PRIMARY KEY (ledger, idempotency_key),
    CONSTRAINT write_requests_operation CHECK (operation IN ('transaction', 'revert', 'hold', 'capture', 'void')),
    CONSTRAINT write_requests_status CHECK (status IN ('pending', 'committed', 'declined', 'failed')),
    CONSTRAINT write_requests_resolved CHECK ((status = 'pending') = (resolved_at IS NULL))
);

-- The reconciler scans pending records oldest first.
CREATE INDEX write_requests_pending ON write_requests (created_at) WHERE status = 'pending';
-- The Requests page lists a ledger newest first.
CREATE INDEX write_requests_ledger_created ON write_requests (ledger, created_at DESC, idempotency_key);

-- A transaction is reverted once and a hold resolved once. A second write with a new key is turned
-- away here, before TigerBeetle, while the first is pending or committed; a declined attempt frees
-- the slot so a corrected request can try again.
CREATE UNIQUE INDEX write_requests_one_revert ON write_requests (ledger, (request ->> 'reverts'))
    WHERE operation = 'revert' AND status IN ('pending', 'committed');
CREATE UNIQUE INDEX write_requests_one_resolution ON write_requests (ledger, (request ->> 'hold'))
    WHERE operation IN ('capture', 'void') AND status IN ('pending', 'committed');

-- +goose Down
DROP TABLE write_requests;
