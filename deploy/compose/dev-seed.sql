-- Development seed of the local stack (deploy/compose/compose.yaml, organizations-seed), until
-- staff create organizations and invite members through the API (docs/plans/auth/plan.md,
-- phase 5). The subjects are the fixed IDs of the dev users in keycloak/realm.json; :'issuer' is
-- the realm's issuer. Running it twice changes nothing.

INSERT INTO organizations (id, alias, name) VALUES
    ('00000000-0000-0000-0000-00000000ac3e', 'acme', 'Acme'),
    ('00000000-0000-0000-0000-0000000061b0', 'globex', 'Globex')
ON CONFLICT DO NOTHING;

-- Organization roles (ADR-0028). Ledger roles need the ledger to exist first.
INSERT INTO grants (organization_id, issuer, subject, role)
SELECT o.id, :'issuer', g.subject, g.role
FROM (VALUES
    ('default', '00000000-0000-4000-8000-000000000ada', 'owner'),
    ('acme', '00000000-0000-4000-8000-000000000ada', 'owner'),
    ('acme', '00000000-0000-4000-8000-0000000ca201', 'admin'),
    ('acme', '00000000-0000-4000-8000-000000000da1', 'member'),
    ('acme', '00000000-0000-4000-8000-000000000e21', 'member'),
    ('globex', '00000000-0000-4000-8000-000000000b0b', 'owner'),
    ('globex', '00000000-0000-4000-8000-0000000ca201', 'member')
) AS g (alias, subject, role)
JOIN organizations o ON o.alias = g.alias
ON CONFLICT DO NOTHING;
