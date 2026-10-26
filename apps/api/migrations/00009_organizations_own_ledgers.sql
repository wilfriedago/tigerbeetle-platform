-- organizations is owned by the access module; the organization_id columns belong to the tables'
-- existing owners (ADR-0027, docs/plans/auth/plan.md section 5).
-- +goose Up
-- An organization is a tenant. Keycloak owns its name, members and invitations; this row is the
-- platform's copy, which the platform's tables point at, plus the status the platform enforces.
-- The alias is unique and immutable in Keycloak, so it links the two.
CREATE TABLE organizations (
    id uuid PRIMARY KEY,
    alias text NOT NULL UNIQUE,
    keycloak_id uuid UNIQUE,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT organizations_alias_format CHECK (alias ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    CONSTRAINT organizations_status CHECK (status IN ('active', 'suspended'))
);

-- Pre-alpha data moves into the organization "default" (ledger.DefaultOrganizationID).
INSERT INTO organizations (id, alias, name) VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'Default');

-- Every table that names a ledger gains the organization, so a ledger is (organization, name).
-- The default only fills existing rows and is dropped at the end.
ALTER TABLE ledgers ADD COLUMN organization_id uuid NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES organizations (id);
ALTER TABLE ledger_assets ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE account_rules ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE event_types ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE addresses ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE accounts ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE write_requests ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE transactions ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE holds ADD COLUMN organization_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';

-- Drop every key that names a ledger without its organization, children first.
ALTER TABLE accounts DROP CONSTRAINT accounts_address_fk;
ALTER TABLE accounts DROP CONSTRAINT accounts_asset_fk;
ALTER TABLE accounts DROP CONSTRAINT accounts_address_asset_unique;
ALTER TABLE ledger_assets DROP CONSTRAINT ledger_assets_ledger_fkey;
ALTER TABLE account_rules DROP CONSTRAINT account_rules_ledger_fkey;
ALTER TABLE event_types DROP CONSTRAINT event_types_ledger_fkey;
ALTER TABLE event_types DROP CONSTRAINT event_types_code_unique;
ALTER TABLE addresses DROP CONSTRAINT addresses_ledger_fkey;
ALTER TABLE write_requests DROP CONSTRAINT write_requests_ledger_fkey;
ALTER TABLE transactions DROP CONSTRAINT transactions_ledger_fkey;
ALTER TABLE holds DROP CONSTRAINT holds_ledger_fkey;
ALTER TABLE ledger_assets DROP CONSTRAINT ledger_assets_pkey;
ALTER TABLE account_rules DROP CONSTRAINT account_rules_pkey;
ALTER TABLE event_types DROP CONSTRAINT event_types_pkey;
ALTER TABLE addresses DROP CONSTRAINT addresses_pkey;
ALTER TABLE write_requests DROP CONSTRAINT write_requests_pkey;
ALTER TABLE ledgers DROP CONSTRAINT ledgers_pkey;

-- The same keys with the organization first.
ALTER TABLE ledgers ADD PRIMARY KEY (organization_id, name);
ALTER TABLE ledger_assets ADD PRIMARY KEY (organization_id, ledger, asset);
ALTER TABLE ledger_assets ADD CONSTRAINT ledger_assets_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);
ALTER TABLE account_rules ADD PRIMARY KEY (organization_id, ledger, pattern);
ALTER TABLE account_rules ADD CONSTRAINT account_rules_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);
ALTER TABLE event_types ADD PRIMARY KEY (organization_id, ledger, name);
ALTER TABLE event_types ADD CONSTRAINT event_types_code_unique UNIQUE (organization_id, ledger, code);
ALTER TABLE event_types ADD CONSTRAINT event_types_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);
ALTER TABLE addresses ADD PRIMARY KEY (organization_id, ledger, address);
ALTER TABLE addresses ADD CONSTRAINT addresses_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);
ALTER TABLE accounts ADD CONSTRAINT accounts_address_asset_unique UNIQUE (organization_id, ledger, address, asset);
ALTER TABLE accounts ADD CONSTRAINT accounts_address_fk
    FOREIGN KEY (organization_id, ledger, address) REFERENCES addresses (organization_id, ledger, address);
ALTER TABLE accounts ADD CONSTRAINT accounts_asset_fk
    FOREIGN KEY (organization_id, ledger, asset) REFERENCES ledger_assets (organization_id, ledger, asset);
ALTER TABLE write_requests ADD PRIMARY KEY (organization_id, ledger, idempotency_key);
ALTER TABLE write_requests ADD CONSTRAINT write_requests_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);
ALTER TABLE transactions ADD CONSTRAINT transactions_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);
ALTER TABLE holds ADD CONSTRAINT holds_ledger_fkey
    FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name);

-- Indexes that lead with the ledger lead with the organization too.
DROP INDEX accounts_ledger_asset;
CREATE INDEX accounts_ledger_asset ON accounts (organization_id, ledger, asset);
DROP INDEX write_requests_ledger_created;
CREATE INDEX write_requests_ledger_created ON write_requests (organization_id, ledger, created_at DESC, idempotency_key);
DROP INDEX write_requests_one_revert;
CREATE UNIQUE INDEX write_requests_one_revert ON write_requests (organization_id, ledger, (request ->> 'reverts'))
    WHERE operation = 'revert' AND status IN ('pending', 'committed');
DROP INDEX write_requests_one_resolution;
CREATE UNIQUE INDEX write_requests_one_resolution ON write_requests (organization_id, ledger, (request ->> 'hold'))
    WHERE operation IN ('capture', 'void') AND status IN ('pending', 'committed');
DROP INDEX transactions_ledger_id;
CREATE INDEX transactions_ledger_id ON transactions (organization_id, ledger, id DESC);
DROP INDEX transactions_ledger_reference;
CREATE INDEX transactions_ledger_reference ON transactions (organization_id, ledger, reference) WHERE reference IS NOT NULL;
DROP INDEX holds_ledger_id;
CREATE INDEX holds_ledger_id ON holds (organization_id, ledger, id DESC);
DROP INDEX holds_ledger_status;
CREATE INDEX holds_ledger_status ON holds (organization_id, ledger, status, id DESC);

ALTER TABLE ledgers ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE ledger_assets ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE account_rules ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE event_types ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE addresses ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE accounts ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE write_requests ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE transactions ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE holds ALTER COLUMN organization_id DROP DEFAULT;

-- +goose Down
-- Going back is only possible while every ledger is in the default organization.
DROP INDEX holds_ledger_status;
CREATE INDEX holds_ledger_status ON holds (ledger, status, id DESC);
DROP INDEX holds_ledger_id;
CREATE INDEX holds_ledger_id ON holds (ledger, id DESC);
DROP INDEX transactions_ledger_reference;
CREATE INDEX transactions_ledger_reference ON transactions (ledger, reference) WHERE reference IS NOT NULL;
DROP INDEX transactions_ledger_id;
CREATE INDEX transactions_ledger_id ON transactions (ledger, id DESC);
DROP INDEX write_requests_one_resolution;
CREATE UNIQUE INDEX write_requests_one_resolution ON write_requests (ledger, (request ->> 'hold'))
    WHERE operation IN ('capture', 'void') AND status IN ('pending', 'committed');
DROP INDEX write_requests_one_revert;
CREATE UNIQUE INDEX write_requests_one_revert ON write_requests (ledger, (request ->> 'reverts'))
    WHERE operation = 'revert' AND status IN ('pending', 'committed');
DROP INDEX write_requests_ledger_created;
CREATE INDEX write_requests_ledger_created ON write_requests (ledger, created_at DESC, idempotency_key);
DROP INDEX accounts_ledger_asset;
CREATE INDEX accounts_ledger_asset ON accounts (ledger, asset);

ALTER TABLE holds DROP CONSTRAINT holds_ledger_fkey;
ALTER TABLE transactions DROP CONSTRAINT transactions_ledger_fkey;
ALTER TABLE write_requests DROP CONSTRAINT write_requests_ledger_fkey;
ALTER TABLE write_requests DROP CONSTRAINT write_requests_pkey;
ALTER TABLE accounts DROP CONSTRAINT accounts_asset_fk;
ALTER TABLE accounts DROP CONSTRAINT accounts_address_fk;
ALTER TABLE accounts DROP CONSTRAINT accounts_address_asset_unique;
ALTER TABLE addresses DROP CONSTRAINT addresses_ledger_fkey;
ALTER TABLE addresses DROP CONSTRAINT addresses_pkey;
ALTER TABLE event_types DROP CONSTRAINT event_types_ledger_fkey;
ALTER TABLE event_types DROP CONSTRAINT event_types_code_unique;
ALTER TABLE event_types DROP CONSTRAINT event_types_pkey;
ALTER TABLE account_rules DROP CONSTRAINT account_rules_ledger_fkey;
ALTER TABLE account_rules DROP CONSTRAINT account_rules_pkey;
ALTER TABLE ledger_assets DROP CONSTRAINT ledger_assets_ledger_fkey;
ALTER TABLE ledger_assets DROP CONSTRAINT ledger_assets_pkey;
ALTER TABLE ledgers DROP CONSTRAINT ledgers_pkey;

ALTER TABLE ledgers ADD PRIMARY KEY (name);
ALTER TABLE ledger_assets ADD PRIMARY KEY (ledger, asset);
ALTER TABLE ledger_assets ADD CONSTRAINT ledger_assets_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);
ALTER TABLE account_rules ADD PRIMARY KEY (ledger, pattern);
ALTER TABLE account_rules ADD CONSTRAINT account_rules_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);
ALTER TABLE event_types ADD PRIMARY KEY (ledger, name);
ALTER TABLE event_types ADD CONSTRAINT event_types_code_unique UNIQUE (ledger, code);
ALTER TABLE event_types ADD CONSTRAINT event_types_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);
ALTER TABLE addresses ADD PRIMARY KEY (ledger, address);
ALTER TABLE addresses ADD CONSTRAINT addresses_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);
ALTER TABLE accounts ADD CONSTRAINT accounts_address_asset_unique UNIQUE (ledger, address, asset);
ALTER TABLE accounts ADD CONSTRAINT accounts_address_fk FOREIGN KEY (ledger, address) REFERENCES addresses (ledger, address);
ALTER TABLE accounts ADD CONSTRAINT accounts_asset_fk FOREIGN KEY (ledger, asset) REFERENCES ledger_assets (ledger, asset);
ALTER TABLE write_requests ADD PRIMARY KEY (ledger, idempotency_key);
ALTER TABLE write_requests ADD CONSTRAINT write_requests_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);
ALTER TABLE transactions ADD CONSTRAINT transactions_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);
ALTER TABLE holds ADD CONSTRAINT holds_ledger_fkey FOREIGN KEY (ledger) REFERENCES ledgers (name);

ALTER TABLE holds DROP COLUMN organization_id;
ALTER TABLE transactions DROP COLUMN organization_id;
ALTER TABLE write_requests DROP COLUMN organization_id;
ALTER TABLE accounts DROP COLUMN organization_id;
ALTER TABLE addresses DROP COLUMN organization_id;
ALTER TABLE event_types DROP COLUMN organization_id;
ALTER TABLE account_rules DROP COLUMN organization_id;
ALTER TABLE ledger_assets DROP COLUMN organization_id;
ALTER TABLE ledgers DROP COLUMN organization_id;
DROP TABLE organizations;
