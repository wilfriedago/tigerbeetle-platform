-- Owned by the access module (ADR-0027).
-- +goose Up
-- A grant gives a principal (issuer and subject) a role in an organization, or on one of its
-- ledgers. Membership itself is Keycloak's (ADR-0026); a grant only says what a member may do.
CREATE TABLE grants (
    organization_id uuid NOT NULL REFERENCES organizations (id),
    issuer text NOT NULL,
    subject text NOT NULL,
    -- NULL for a role in the whole organization, a ledger name for a role on that ledger.
    ledger text,
    role text NOT NULL,
    granted_by_issuer text,
    granted_by_subject text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT grants_ledger_fkey FOREIGN KEY (organization_id, ledger) REFERENCES ledgers (organization_id, name),
    CONSTRAINT grants_role CHECK (
        (ledger IS NULL AND role IN ('member', 'admin', 'owner'))
        OR (ledger IS NOT NULL AND role IN ('viewer', 'operator', 'admin'))
    )
);

-- One role per principal in an organization, and one per principal on a ledger.
CREATE UNIQUE INDEX grants_one_role ON grants (organization_id, issuer, subject, COALESCE(ledger, ''));

-- +goose Down
DROP TABLE grants;
