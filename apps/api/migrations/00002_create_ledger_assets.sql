-- Owned by the bookkeeping module (internal/modules/bookkeeping, plan section 4.7).
-- +goose Up
-- One TigerBeetle ledger number per (ledger, asset) pair (ADR-0005). TigerBeetle rejects a transfer
-- between accounts with different numbers, so assets and namespaces cannot mix inside the cluster.
-- Numbers are allocated lazily, never reused, and rows are never updated or deleted.
CREATE TABLE ledger_assets (
    ledger text NOT NULL REFERENCES ledgers (name),
    -- Canonical CODE/SCALE form, so EUR and EUR/0 cannot create two rows.
    asset text NOT NULL,
    tb_ledger integer GENERATED ALWAYS AS IDENTITY,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ledger, asset),
    CONSTRAINT ledger_assets_tb_ledger_positive CHECK (tb_ledger > 0),
    CONSTRAINT ledger_assets_tb_ledger_unique UNIQUE (tb_ledger),
    CONSTRAINT ledger_assets_asset_format CHECK (asset ~ '^[A-Z][A-Z0-9]{1,15}/[0-9]{1,2}$')
);

-- +goose Down
DROP TABLE ledger_assets;
