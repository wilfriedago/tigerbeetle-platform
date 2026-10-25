-- +goose Up
-- Committed transactions and their context (ADR-0004, ADR-0007). Amounts are not stored: the API
-- reads them from TigerBeetle with lookup_transfers on the leg IDs. committed_at is the TigerBeetle
-- timestamp of the chain, copied here so lists can filter by time.
CREATE TABLE transactions (
    id uuid PRIMARY KEY,
    ledger text NOT NULL REFERENCES ledgers (name),
    reference text,
    event_type text NOT NULL,
    code integer NOT NULL,
    business_time timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key text NOT NULL,
    request_id text,
    reverts uuid UNIQUE REFERENCES transactions (id),
    committed_at timestamptz NOT NULL,
    tb_timestamp bigint NOT NULL,
    CONSTRAINT transactions_reference_length CHECK (length(reference) <= 255)
);

CREATE INDEX transactions_ledger_id ON transactions (ledger, id DESC);
CREATE INDEX transactions_ledger_reference ON transactions (ledger, reference) WHERE reference IS NOT NULL;

-- One row per posting: which TigerBeetle transfer and which accounts, never the amount.
CREATE TABLE transaction_postings (
    transaction_id uuid NOT NULL REFERENCES transactions (id),
    position smallint NOT NULL,
    transfer_id uuid NOT NULL UNIQUE,
    source_account uuid NOT NULL REFERENCES accounts (id),
    destination_account uuid NOT NULL REFERENCES accounts (id),
    PRIMARY KEY (transaction_id, position)
);

CREATE INDEX transaction_postings_source ON transaction_postings (source_account, transaction_id);
CREATE INDEX transaction_postings_destination ON transaction_postings (destination_account, transaction_id);

-- +goose Down
DROP TABLE transaction_postings;
DROP TABLE transactions;
