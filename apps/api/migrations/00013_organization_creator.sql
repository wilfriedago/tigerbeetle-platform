-- Owned by the access module (ADR-0027, ADR-0028).
-- +goose Up
-- Who created an organization: a staff member, or nobody for the seeded ones. The name is kept as
-- it was then, for display; the issuer and subject identify the person.
ALTER TABLE organizations ADD COLUMN created_by_issuer text;
ALTER TABLE organizations ADD COLUMN created_by_subject text;
ALTER TABLE organizations ADD COLUMN created_by_name text;
-- Why staff suspended the organization, shown to its members.
ALTER TABLE organizations ADD COLUMN suspension_reason text;

-- The name of who granted a role, as it was then, for the member lists.
ALTER TABLE grants ADD COLUMN granted_by_name text;

-- +goose Down
ALTER TABLE grants DROP COLUMN granted_by_name;
ALTER TABLE organizations DROP COLUMN suspension_reason;
ALTER TABLE organizations DROP COLUMN created_by_name;
ALTER TABLE organizations DROP COLUMN created_by_subject;
ALTER TABLE organizations DROP COLUMN created_by_issuer;
