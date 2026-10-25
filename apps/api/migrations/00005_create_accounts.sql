-- +goose Up
-- An address is what clients name ("users:1234:wallet"); it carries metadata. The "C" collation
-- makes comparisons bytewise, so a whole-segment prefix becomes the exact range
-- [prefix || ':', prefix || ';') and uses the primary key index. ';' is the byte after ':'.
CREATE TABLE addresses (
    ledger text NOT NULL REFERENCES ledgers (name),
    address text COLLATE "C" NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ledger, address),
    CONSTRAINT addresses_format CHECK (address ~ '^[a-zA-Z0-9_-]+(:[a-zA-Z0-9_-]+)*$' AND length(address) <= 255)
);

-- One TigerBeetle account per (address, asset) (ADR-0006). The ID is minted with the client's ID()
-- helper and stored big-endian, so ORDER BY id is creation order (ADR-0002). Flags, code and the
-- rule that matched are frozen here at creation and sent unchanged on every create_accounts, which
-- keeps the healing path idempotent.
CREATE TABLE accounts (
    id uuid PRIMARY KEY,
    ledger text NOT NULL,
    address text COLLATE "C" NOT NULL,
    asset text NOT NULL,
    tb_ledger integer NOT NULL,
    code integer NOT NULL,
    balance_limit text NOT NULL,
    history boolean NOT NULL,
    rule_pattern text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT accounts_address_asset_unique UNIQUE (ledger, address, asset),
    CONSTRAINT accounts_address_fk FOREIGN KEY (ledger, address) REFERENCES addresses (ledger, address),
    CONSTRAINT accounts_asset_fk FOREIGN KEY (ledger, asset) REFERENCES ledger_assets (ledger, asset),
    CONSTRAINT accounts_code_range CHECK (code BETWEEN 1 AND 65535),
    CONSTRAINT accounts_limit CHECK (
        balance_limit IN ('none', 'debits_must_not_exceed_credits', 'credits_must_not_exceed_debits')
    )
);

CREATE INDEX accounts_ledger_asset ON accounts (ledger, asset);

-- +goose Down
DROP TABLE accounts;
DROP TABLE addresses;
