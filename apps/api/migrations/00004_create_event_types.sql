-- Owned by the ledgers module (internal/modules/ledgers, plan section 4.7).
-- +goose Up
-- Business event types and the TigerBeetle transfer code each one is written as (ADR-0007). The
-- table is append-only: a code never changes meaning. Code 1 is the built-in "default" type.
CREATE TABLE event_types (
    ledger text NOT NULL REFERENCES ledgers (name),
    name text NOT NULL,
    code integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ledger, name),
    CONSTRAINT event_types_code_unique UNIQUE (ledger, code),
    CONSTRAINT event_types_code_range CHECK (code BETWEEN 2 AND 65535),
    CONSTRAINT event_types_name_format CHECK (name ~ '^[a-z][a-z0-9_.-]{0,62}$')
);

-- +goose Down
DROP TABLE event_types;
