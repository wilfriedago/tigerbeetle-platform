-- Owned by the ledgers module (internal/modules/ledgers, plan section 4.7).
-- +goose Up
-- Address-pattern rules decide the flags and code of new TigerBeetle accounts (ADR-0006). The
-- built-in "*" and "world" rules exist in code; a stored row with the same pattern replaces them.
-- Changing a rule only affects accounts created afterwards: flags are frozen on accounts rows.
CREATE TABLE account_rules (
    ledger text NOT NULL REFERENCES ledgers (name),
    pattern text NOT NULL,
    balance_limit text NOT NULL,
    history boolean NOT NULL DEFAULT false,
    code integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ledger, pattern),
    CONSTRAINT account_rules_limit CHECK (
        balance_limit IN ('none', 'debits_must_not_exceed_credits', 'credits_must_not_exceed_debits')
    ),
    CONSTRAINT account_rules_code_range CHECK (code BETWEEN 1 AND 65535)
);

-- +goose Down
DROP TABLE account_rules;
