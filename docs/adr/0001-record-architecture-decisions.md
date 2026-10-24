# 1. Record architecture decisions

Status: Accepted
Date: 2026-10-24

## Context and Problem Statement

This repository puts a Go API and a web console in front of TigerBeetle, in a
Formance-style shape, with Postgres holding metadata. Most of the hard choices
were made before the first line of feature code existed: TigerBeetle is the
only store for money, assets map to TigerBeetle ledger numbers, IDs come from
the client's time-ordered ID() helper, every write carries an Idempotency-Key,
and so on. Each of these choices has consequences that are not obvious from the
code alone. A reader who sees `INSERT ... ON CONFLICT DO NOTHING` followed by a
re-select in the account creation path cannot tell from the SQL that this is
the chosen answer to lazy account creation, or why `exists` from
`create_accounts` is treated as success.

The project is pre-alpha and the team is small. Engineers joining later, some
new to Go and most new to TigerBeetle, need to know not only what was decided
but why, what else was considered, and what it would take to change course.
Without a record, decisions get relitigated in pull requests, or silently
reversed by a well-meaning refactor.

We need a lightweight, versioned way to record architecturally significant
decisions next to the code they govern.

## Decision Drivers

- Decisions must survive turnover and be readable without the original author.
- The record must live in git, be reviewed in pull requests, and be diffable.
- The format must be cheap enough that people actually write it; a decision
  that takes an afternoon to document will not be documented.
- Each record should be small and focused so that it can be read in minutes
  and linked from code comments, `docs/architecture.md` and `AGENTS.md`.
- Changing a decision must be an explicit act with a trail, not a quiet edit.
- The repository is Apache-2.0 and public; the records double as design notes
  for outside contributors.

## Considered Options

1. Architecture Decision Records in MADR format, one file per decision under
   `docs/adr`.
2. A single long design document (`docs/architecture.md` only) that is edited
   in place as decisions change.
3. Decisions recorded in GitHub issues or discussions and linked from the
   README.
4. No formal record; rely on commit messages and code comments.

## Decision Outcome

Chosen option: 1, Architecture Decision Records in MADR format, because it is
the only option that keeps each decision small, reviewable, versioned with the
code, and stable once accepted. A single design document (option 2) is kept as
well, but as a current-state summary that points at ADRs, not as the record of
why. Issues (option 3) are not versioned with the code and are easy to lose
behind the default filters. Commit messages (option 4) record what changed,
not the alternatives that were rejected.

The rules:
- Location: `docs/adr/`. One Markdown file per decision.
- Naming: a four-digit zero-padded number, a hyphen, and a short kebab-case
  slug, for example `0006-lazy-account-creation-and-address-rules.md`. Numbers are allocated in
  order and never reused, even when an ADR is superseded.
- Layout: the MADR sections used in this file, in this order: title with the
  number, `Status` and `Date` lines, `Context and Problem Statement`,
  `Decision Drivers`, `Considered Options`, `Decision Outcome` with a
  `Consequences` subsection listing `Good:` and `Bad:` bullets, and
  `More Information` with links to related ADRs, repository paths and the
  external documentation relied on.
- One decision per file. If a change touches two decisions, write two ADRs
  and cross-link them.
- Status: an ADR is merged with status `Accepted`. We do not merge drafts;
  the pull request is the proposal stage. The other statuses used are
  `Superseded by [ADR-NNNN](NNNN-slug.md)` and `Deprecated`.
- Style: plain Markdown, ASCII punctuation, no emojis, SQL keywords in
  uppercase, 60 to 160 lines. Write for an engineer new to the codebase; name
  real paths such as `apps/api/internal/app/ports.go` rather than describing
  them.
- Facts about TigerBeetle must come from https://docs.tigerbeetle.com or from
  the pinned server and client source, and the `More Information` section
  must cite the page relied on.

When to write one: any change that alters a decision listed in
`docs/architecture.md` or in `AGENTS.md` requires an ADR in the same pull
request. Typical triggers are a new storage location for any kind of data, a
change to how IDs are minted or encoded, a change to the idempotency or
reconciliation protocol, a new public API shape that diverges from the
Formance-style surface, a change to the dependency direction enforced by the
depguard rules in `apps/api/.golangci.yml`, or a version policy change such as
the one enforced by `scripts/check-versions.sh`. Routine work (a new endpoint
that follows existing patterns, a dependency bump, a bug fix) does not need an
ADR.

How to supersede: never rewrite an accepted ADR's decision. Write a new ADR
with the next number, explain in its context what changed since the old one,
and link back to it. Then edit only the `Status` line of the old ADR to
`Superseded by [ADR-NNNN](NNNN-slug.md)` and update `docs/architecture.md`
and `AGENTS.md` to point at the new ADR. Fixing typos, broken links or wrong
paths in an accepted ADR is allowed without a new ADR. Changing what it
decides is not.

### Consequences

- Good: a newcomer can read `docs/adr` in order and understand how the system
  reached its current shape, including the options that were rejected.
- Good: pull request reviews can ask "which ADR covers this?" instead of
  re-arguing settled questions.
- Good: superseding leaves a trail, so an old constraint can be traced to the
  moment it was lifted and the reason given.
- Good: the format is stable enough that ADRs can be linked from code comments
  and from the OpenAPI spec description without the links rotting.
- Bad: there is a documentation cost on every significant change, and the
  rule that an ADR ships in the same pull request can slow down work that is
  exploratory by nature.
- Bad: ADRs record intent, not behaviour. If an ADR and the code disagree, the
  code is what runs; keeping them aligned is a review discipline, not a tool
  check.
- Bad: the numbering is a global sequence, so two concurrent pull requests can
  claim the same number and one of them has to renumber before merge.

## More Information

- The first set of ADRs, written on the same day as this one, records the
  decisions the project started from:
  [ADR-0002](0002-tigerbeetle-source-of-truth-postgres-context.md) through
  [ADR-0010](0010-holds-as-two-phase-transfers.md). `docs/architecture.md`
  lists them with one line each.
- `docs/architecture.md` is the current-state summary that links to the ADRs
  and lists the decisions whose change requires a new ADR.
- `AGENTS.md` carries the same list for automated contributors.
- MADR, the format these records follow: https://adr.github.io/madr/
- Michael Nygard's original description of ADRs:
  https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions
- TigerBeetle documentation, the only accepted source for TigerBeetle
  behaviour cited in an ADR: https://docs.tigerbeetle.com
