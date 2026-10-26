-- name: GrantsOf :many
-- The roles of one principal in one organization: its organization role (ledger NULL) and its
-- ledger roles.
SELECT ledger, role
FROM grants
WHERE organization_id = $1 AND issuer = $2 AND subject = $3;

-- name: GrantsIn :many
-- Every grant in one organization, the organization role of each holder before its ledger roles.
SELECT issuer, subject, ledger, role
FROM grants
WHERE organization_id = $1
ORDER BY issuer, subject, ledger NULLS FIRST;

-- name: PutGrant :exec
-- One role per holder in the organization and one per ledger: a new role replaces the old one.
INSERT INTO grants (organization_id, issuer, subject, ledger, role, granted_by_issuer, granted_by_subject)
VALUES ($1, $2, $3, sqlc.narg(ledger), $4, sqlc.narg(granted_by_issuer), sqlc.narg(granted_by_subject))
ON CONFLICT (organization_id, issuer, subject, COALESCE(ledger, '')) DO UPDATE
SET role = EXCLUDED.role, granted_by_issuer = EXCLUDED.granted_by_issuer,
    granted_by_subject = EXCLUDED.granted_by_subject, created_at = now();

-- name: RevokeGrant :execrows
DELETE FROM grants
WHERE organization_id = $1 AND issuer = $2 AND subject = $3 AND COALESCE(ledger, '') = sqlc.arg(ledger);

-- name: RevokeGrants :exec
DELETE FROM grants
WHERE organization_id = $1 AND issuer = $2 AND subject = $3;

-- name: CountOwners :one
SELECT count(*)
FROM grants
WHERE organization_id = $1 AND ledger IS NULL AND role = 'owner';
