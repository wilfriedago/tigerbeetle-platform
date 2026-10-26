-- Prefix filters match whole segments: the prefix itself, or the bytewise range
-- [prefix || ':', prefix || ';'). Addresses use the "C" collation, so the range is exact and
-- served by the primary key index, and '_' in an address is never read as a LIKE wildcard.

-- name: EnsureAssets :exec
INSERT INTO ledger_assets (organization_id, ledger, asset)
SELECT sqlc.arg(organization_id)::uuid, $1, unnest(sqlc.arg(assets)::text[])
ON CONFLICT (organization_id, ledger, asset) DO NOTHING;

-- name: SelectAssets :many
SELECT asset, tb_ledger, created_at
FROM ledger_assets
WHERE organization_id = sqlc.arg(organization_id) AND ledger = $1 AND asset = ANY(sqlc.arg(assets)::text[]);

-- name: ListAssets :many
SELECT
    la.asset,
    la.tb_ledger,
    la.created_at,
    (SELECT count(*) FROM accounts a WHERE a.organization_id = la.organization_id AND a.ledger = la.ledger AND a.asset = la.asset)::bigint AS accounts
FROM ledger_assets la
WHERE la.organization_id = $2 AND la.ledger = $1
ORDER BY la.created_at, la.asset;

-- name: InsertAddresses :exec
INSERT INTO addresses (organization_id, ledger, address, created_at)
SELECT sqlc.arg(organization_id)::uuid, $1, unnest(sqlc.arg(addresses)::text[]), sqlc.arg(created_at)
ON CONFLICT (organization_id, ledger, address) DO NOTHING;

-- name: InsertAccounts :exec
INSERT INTO accounts (organization_id, id, ledger, address, asset, tb_ledger, code, balance_limit, history, rule_pattern, created_at)
SELECT
    sqlc.arg(organization_id)::uuid,
    unnest(sqlc.arg(ids)::uuid[]),
    sqlc.arg(ledger)::text,
    unnest(sqlc.arg(addresses)::text[]),
    unnest(sqlc.arg(assets)::text[]),
    unnest(sqlc.arg(tb_ledgers)::integer[]),
    unnest(sqlc.arg(codes)::integer[]),
    unnest(sqlc.arg(limits)::text[]),
    unnest(sqlc.arg(histories)::boolean[]),
    unnest(sqlc.arg(rule_patterns)::text[]),
    sqlc.arg(created_at)
ON CONFLICT (organization_id, ledger, address, asset) DO NOTHING;

-- name: FindAccounts :many
-- A superset of the requested (address, asset) pairs; the adapter keeps the exact pairs.
SELECT id, ledger, address, asset, tb_ledger, code, balance_limit, history, rule_pattern, created_at, organization_id
FROM accounts
WHERE organization_id = sqlc.arg(organization_id) AND ledger = sqlc.arg(ledger)
    AND address = ANY(sqlc.arg(addresses)::text[])
    AND asset = ANY(sqlc.arg(assets)::text[]);

-- name: AccountsByID :many
-- tenant-free: account IDs are unique across the platform; the CDC relay reads them for every
-- organization, and other callers pass IDs they read inside their own.
SELECT id, ledger, address, asset, tb_ledger, code, balance_limit, history, rule_pattern, created_at, organization_id
FROM accounts
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AccountsOfAddresses :many
SELECT id, ledger, address, asset, tb_ledger, code, balance_limit, history, rule_pattern, created_at, organization_id
FROM accounts
WHERE organization_id = sqlc.arg(organization_id) AND ledger = $1 AND address = ANY(sqlc.arg(addresses)::text[])
ORDER BY address, asset;

-- name: ListAddresses :many
SELECT a.address, a.metadata, a.created_at
FROM addresses a
WHERE a.organization_id = sqlc.arg(organization_id) AND a.ledger = sqlc.arg(ledger)
    AND a.address > sqlc.arg(after)::text
    AND (
        sqlc.narg(prefix)::text IS NULL
        OR a.address = sqlc.narg(prefix)::text
        OR (a.address >= sqlc.narg(prefix)::text || ':' AND a.address < sqlc.narg(prefix)::text || ';')
    )
    AND (
        sqlc.narg(search)::text IS NULL
        OR (a.address >= sqlc.narg(search)::text AND a.address < sqlc.narg(search)::text || '{')
    )
    AND (
        sqlc.narg(asset)::text IS NULL
        OR EXISTS (SELECT 1 FROM accounts x WHERE x.organization_id = a.organization_id AND x.ledger = a.ledger AND x.address = a.address AND x.asset = sqlc.narg(asset)::text)
    )
ORDER BY a.address
LIMIT sqlc.arg(page_size);

-- name: GetAddress :one
SELECT address, metadata, created_at
FROM addresses
WHERE organization_id = $3 AND ledger = $1 AND address = $2;

-- name: ReplaceAddressMetadata :execrows
UPDATE addresses
SET metadata = $3
WHERE organization_id = $4 AND ledger = $1 AND address = $2;

-- name: AccountsUnder :many
SELECT id, ledger, address, asset, tb_ledger, code, balance_limit, history, rule_pattern, created_at, organization_id
FROM accounts
WHERE organization_id = sqlc.arg(organization_id) AND ledger = sqlc.arg(ledger)
    AND (
        address = sqlc.arg(prefix)::text
        OR (address >= sqlc.arg(prefix)::text || ':' AND address < sqlc.arg(prefix)::text || ';')
    )
ORDER BY address, asset
LIMIT sqlc.arg(max_rows);

-- name: Segments :many
-- The next segment below a prefix; an empty prefix lists the top-level segments.
WITH under AS (
    SELECT
        a.address,
        CASE
            WHEN sqlc.arg(prefix)::text = '' THEN split_part(a.address, ':', 1)
            ELSE split_part(substr(a.address, length(sqlc.arg(prefix)::text) + 2), ':', 1)
        END AS segment
    FROM addresses a
    WHERE a.organization_id = sqlc.arg(organization_id) AND a.ledger = sqlc.arg(ledger)
        AND (
            sqlc.arg(prefix)::text = ''
            OR (a.address >= sqlc.arg(prefix)::text || ':' AND a.address < sqlc.arg(prefix)::text || ';')
        )
)
SELECT
    u.segment::text AS segment,
    count(*)::bigint AS accounts,
    bool_or(u.address = CASE WHEN sqlc.arg(prefix)::text = '' THEN u.segment ELSE sqlc.arg(prefix)::text || ':' || u.segment END)::boolean AS is_account,
    bool_or(u.address <> CASE WHEN sqlc.arg(prefix)::text = '' THEN u.segment ELSE sqlc.arg(prefix)::text || ':' || u.segment END)::boolean AS has_children,
    COALESCE(
        (
            SELECT array_agg(DISTINCT acc.asset ORDER BY acc.asset)
            FROM accounts acc
            JOIN under u2 ON u2.address = acc.address
            WHERE acc.organization_id = sqlc.arg(organization_id) AND acc.ledger = sqlc.arg(ledger) AND u2.segment = u.segment
        ),
        '{}'
    )::text[] AS assets
FROM under u
GROUP BY u.segment
ORDER BY u.segment;

-- name: CountAddresses :one
SELECT count(*)::bigint AS total, count(*) FILTER (WHERE created_at >= sqlc.arg(since))::bigint AS recent
FROM addresses
WHERE organization_id = sqlc.arg(organization_id) AND ledger = sqlc.arg(ledger);
