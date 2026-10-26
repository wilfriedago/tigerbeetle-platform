-- name: GrantsOf :many
-- The roles of one principal in one organization: its organization role (ledger NULL) and its
-- ledger roles.
SELECT ledger, role
FROM grants
WHERE organization_id = $1 AND issuer = $2 AND subject = $3;

-- name: GrantsIn :many
-- Every grant in one organization, the organization role of each holder before its ledger roles.
SELECT issuer, subject, ledger, role, created_at, granted_by_name
FROM grants
WHERE organization_id = $1
ORDER BY issuer, subject, ledger NULLS FIRST;

-- name: PutGrant :exec
-- One role per holder in the organization and one per ledger: a new role replaces the old one.
INSERT INTO grants (organization_id, issuer, subject, ledger, role, granted_by_issuer, granted_by_subject, granted_by_name)
VALUES ($1, $2, $3, sqlc.narg(ledger), $4, sqlc.narg(granted_by_issuer), sqlc.narg(granted_by_subject), sqlc.narg(granted_by_name))
ON CONFLICT (organization_id, issuer, subject, COALESCE(ledger, '')) DO UPDATE
SET role = EXCLUDED.role, granted_by_issuer = EXCLUDED.granted_by_issuer,
    granted_by_subject = EXCLUDED.granted_by_subject, granted_by_name = EXCLUDED.granted_by_name, created_at = now();

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

-- name: OwnersIn :many
-- The owners of several organizations, for the staff's list: names the organizations it reads.
SELECT organization_id, issuer, subject, created_at
FROM grants
WHERE organization_id = ANY(sqlc.arg(organization_ids)::uuid[]) AND ledger IS NULL AND role = 'owner'
ORDER BY organization_id, created_at;

-- name: GrantsAcross :many
-- tenant-free: the caller's own roles in every organization, for their profile and the
-- organization chooser; a principal reads only its own grants.
SELECT o.alias, g.ledger, g.role
FROM grants g
JOIN organizations o ON o.id = g.organization_id
WHERE g.issuer = $1 AND g.subject = $2
ORDER BY o.alias, g.ledger NULLS FIRST;
