-- +goose Up
-- A ledger is a namespace for accounts and transactions (decision 2). Postgres holds its name and
-- metadata only; balances live in TigerBeetle (decision 1).
CREATE TABLE ledgers (
    name text PRIMARY KEY,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ledgers_name_format CHECK (name ~ '^[a-z0-9][a-z0-9_-]{0,62}$')
);

-- +goose Down
DROP TABLE ledgers;
