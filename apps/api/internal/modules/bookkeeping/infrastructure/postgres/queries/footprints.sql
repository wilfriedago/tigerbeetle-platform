-- Reads the ledgers module asks bookkeeping for, through the ports of plan section 4.5.

-- name: LedgerFootprints :many
SELECT
    n.name::text AS ledger,
    COALESCE((SELECT array_agg(la.asset ORDER BY la.asset) FROM ledger_assets la WHERE la.ledger = n.name), '{}')::text[] AS assets,
    (SELECT count(*) FROM addresses a WHERE a.ledger = n.name)::bigint AS accounts
FROM unnest(sqlc.arg(ledgers)::text[]) AS n(name);

-- name: EventTypeUsage :many
SELECT u.event_type, count(*)::bigint AS uses
FROM (
    SELECT t.event_type FROM transactions t WHERE t.ledger = $1
    UNION ALL
    SELECT h.event_type FROM holds h WHERE h.ledger = $1
) u
GROUP BY u.event_type;
