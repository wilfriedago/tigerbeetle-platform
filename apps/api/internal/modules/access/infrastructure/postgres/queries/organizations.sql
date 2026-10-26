-- organizations is the access module's own table and the tenant itself, not a tenant table.

-- name: OrganizationByAlias :one
SELECT id, alias, keycloak_id, name, status, created_at
FROM organizations
WHERE alias = $1;

-- name: OrganizationsByAlias :many
SELECT id, alias, keycloak_id, name, status, created_at
FROM organizations
WHERE alias = ANY(sqlc.arg(aliases)::text[])
ORDER BY name;

-- name: ListOrganizations :many
SELECT id, alias, keycloak_id, name, status, created_at
FROM organizations
WHERE alias > sqlc.arg(after)
ORDER BY alias
LIMIT sqlc.arg(max_rows);

-- name: SaveOrganization :one
-- Creates the organization, or completes the one with the same alias: a retried creation fills in
-- the Keycloak ID and returns the existing row. uuidv7() is time ordered (Postgres 18).
INSERT INTO organizations (id, alias, keycloak_id, name, created_by_issuer, created_by_subject)
VALUES (uuidv7(), sqlc.arg(alias), sqlc.narg(keycloak_id), sqlc.arg(name), sqlc.narg(created_by_issuer), sqlc.narg(created_by_subject))
ON CONFLICT (alias) DO UPDATE SET keycloak_id = COALESCE(organizations.keycloak_id, EXCLUDED.keycloak_id)
RETURNING id, alias, keycloak_id, name, status, created_at;

-- name: SetOrganizationKeycloakID :exec
UPDATE organizations SET keycloak_id = sqlc.arg(keycloak_id)
WHERE alias = sqlc.arg(alias) AND keycloak_id IS NULL;

-- name: RenameOrganization :one
UPDATE organizations SET name = sqlc.arg(name)
WHERE alias = sqlc.arg(alias)
RETURNING id, alias, keycloak_id, name, status, created_at;

-- name: SetOrganizationStatus :one
UPDATE organizations SET status = sqlc.arg(status)
WHERE alias = sqlc.arg(alias)
RETURNING id, alias, keycloak_id, name, status, created_at;
