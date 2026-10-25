-- Owned by the bookkeeping module (internal/modules/bookkeeping, plan section 4.7).
-- +goose Up
-- Holds are TigerBeetle pending transfers (ADR-0010). The amounts live in TigerBeetle; this row holds
-- the context and the resolution. Expired is derived at read time from created_at and the timeout,
-- and recorded here once a capture or void meets pending_transfer_expired.
CREATE TABLE holds (
    id uuid PRIMARY KEY,
    ledger text NOT NULL REFERENCES ledgers (name),
    pending_transfer_id uuid NOT NULL UNIQUE,
    source_account uuid NOT NULL REFERENCES accounts (id),
    destination_account uuid NOT NULL REFERENCES accounts (id),
    reference text,
    event_type text NOT NULL,
    code integer NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    timeout_seconds bigint NOT NULL DEFAULT 0,
    idempotency_key text NOT NULL,
    request_id text,
    created_at timestamptz NOT NULL,
    tb_timestamp bigint NOT NULL,
    status text NOT NULL DEFAULT 'open',
    resolution_transfer_id uuid UNIQUE,
    resolved_at timestamptz,
    CONSTRAINT holds_status CHECK (status IN ('open', 'captured', 'voided', 'expired')),
    CONSTRAINT holds_timeout_range CHECK (timeout_seconds BETWEEN 0 AND 4294967295),
    CONSTRAINT holds_reference_length CHECK (length(reference) <= 255)
);

CREATE INDEX holds_ledger_id ON holds (ledger, id DESC);
CREATE INDEX holds_ledger_status ON holds (ledger, status, id DESC);
CREATE INDEX holds_source ON holds (source_account);
CREATE INDEX holds_destination ON holds (destination_account);

-- +goose Down
DROP TABLE holds;
