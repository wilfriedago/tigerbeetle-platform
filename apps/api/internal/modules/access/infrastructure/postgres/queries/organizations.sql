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
