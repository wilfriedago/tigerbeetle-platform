-- Owned by the access module (ADR-0027).
-- +goose Up
-- Who created an organization: a staff member, or nobody for the seeded ones.
ALTER TABLE organizations ADD COLUMN created_by_issuer text;
ALTER TABLE organizations ADD COLUMN created_by_subject text;

-- +goose Down
ALTER TABLE organizations DROP COLUMN created_by_subject;
ALTER TABLE organizations DROP COLUMN created_by_issuer;
