-- name: GrantsOf :many
-- The roles of one principal in one organization: its organization role (ledger NULL) and its
-- ledger roles.
SELECT ledger, role
FROM grants
WHERE organization_id = $1 AND issuer = $2 AND subject = $3;
