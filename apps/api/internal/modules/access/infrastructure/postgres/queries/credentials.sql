-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, organization_id, name, secret_hash, expires_at, created_by_issuer, created_by_subject, created_by_name)
VALUES ($1, $2, $3, $4, sqlc.narg(expires_at), sqlc.narg(created_by_issuer), sqlc.narg(created_by_subject), sqlc.narg(created_by_name));

-- name: APIKeys :many
SELECT id, name, expires_at, last_used_at, revoked_at, created_at, created_by_name
FROM api_keys
WHERE organization_id = $1
ORDER BY revoked_at IS NOT NULL, name;

-- name: APIKey :one
SELECT id, name, expires_at, last_used_at, revoked_at, created_at, created_by_name
FROM api_keys
WHERE organization_id = $1 AND id = $2;

-- name: APIKeyForAuthentication :one
-- tenant-free: authentication finds the key by the ID it carries, before any tenant is known; the
-- key then names its organization.
SELECT k.secret_hash, k.name, k.expires_at, k.revoked_at, k.last_used_at, o.id AS organization_id, o.alias, o.status, o.suspension_reason
FROM api_keys k
JOIN organizations o ON o.id = k.organization_id
WHERE k.id = $1;

-- name: TouchAPIKey :exec
-- tenant-free: by the key's own ID, at most once a minute.
UPDATE api_keys SET last_used_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND (last_used_at IS NULL OR last_used_at < sqlc.arg(at)::timestamptz - interval '1 minute');

-- name: RevokeAPIKey :execrows
UPDATE api_keys SET revoked_at = sqlc.arg(at)
WHERE organization_id = sqlc.arg(organization_id) AND id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: CreateMachineClient :exec
INSERT INTO machine_clients (client_id, organization_id, keycloak_id, subject, name, created_by_issuer, created_by_subject, created_by_name)
VALUES ($1, $2, $3, $4, $5, sqlc.narg(created_by_issuer), sqlc.narg(created_by_subject), sqlc.narg(created_by_name));

-- name: MachineClients :many
SELECT client_id, keycloak_id, subject, name, secret_set_at, last_token_at, deleted_at, created_at, created_by_name
FROM machine_clients
WHERE organization_id = $1
ORDER BY deleted_at IS NOT NULL, name;

-- name: MachineClient :one
SELECT client_id, keycloak_id, subject, name, secret_set_at, last_token_at, deleted_at, created_at, created_by_name
FROM machine_clients
WHERE organization_id = $1 AND client_id = $2;

-- name: MachineClientForAuthentication :one
-- tenant-free: authentication finds the client by its token's client ID; the row names its
-- organization.
SELECT m.subject, m.name, m.deleted_at, o.id AS organization_id, o.alias, o.status, o.suspension_reason
FROM machine_clients m
JOIN organizations o ON o.id = m.organization_id
WHERE m.client_id = $1;

-- name: TouchMachineClient :exec
-- tenant-free: by the client's own ID, at most once a minute.
UPDATE machine_clients SET last_token_at = sqlc.arg(at)
WHERE client_id = sqlc.arg(client_id) AND (last_token_at IS NULL OR last_token_at < sqlc.arg(at)::timestamptz - interval '1 minute');

-- name: SetMachineClientSecretSetAt :execrows
UPDATE machine_clients SET secret_set_at = sqlc.arg(at)
WHERE organization_id = sqlc.arg(organization_id) AND client_id = sqlc.arg(client_id) AND deleted_at IS NULL;

-- name: DeleteMachineClient :execrows
UPDATE machine_clients SET deleted_at = sqlc.arg(at)
WHERE organization_id = sqlc.arg(organization_id) AND client_id = sqlc.arg(client_id) AND deleted_at IS NULL;
