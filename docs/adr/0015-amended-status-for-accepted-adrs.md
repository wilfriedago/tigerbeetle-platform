# 15. Amended status for accepted ADRs

Status: Accepted
Date: 2026-10-25

## Context and Problem Statement

ADR-0001 knows three statuses: `Accepted`, `Superseded by [ADR-NNNN](NNNN-slug.md)` and
`Deprecated`. It forbids rewriting an accepted decision and allows one edit to an accepted ADR:
its `Status` line, when a newer ADR supersedes it as a whole.

Several later ADRs replaced only part of an older one:

- ADR-0011 replaced the Biome parts of ADR-0003 and ADR-0009.
- ADR-0012 replaced the lefthook part of ADR-0009.
- ADR-0013 narrowed ADR-0004's "every write requires an `Idempotency-Key`" to the five writes
  that move money, and added `ErrRejected` and `ErrConflict` to its four sentinel errors. It also
  made the idempotency record of ADR-0007 store the canonical request, not only a body hash.

ADR-0003, ADR-0004, ADR-0007 and ADR-0009 still say `Accepted` and nothing more. A reader who
opens one of them, or a coding agent that reads it and acts on it, has no sign that some of its
sentences no longer hold. The newer ADR names the part it replaces, but only someone who already
knows to look for it finds it. The migration to bounded-context modules and new transports, in
the ADRs that follow this one, amends several more records in the same partial way.

## Decision Drivers

- Anyone who opens an ADR must see in its first lines whether all of it still applies.
- ADR-0001's guarantees stay: an accepted decision is never rewritten, and changing one is an
  explicit act with a trail.
- One line, no new file, no tool. ADRs are cheap to write only if their rules are.
- Plain text that `grep` can find, so a person or an agent can list every amended record.
- One ADR can be amended more than once; the status must show all of them.

## Considered Options

1. A new status line, `Accepted, amended by [ADR-NNNN](NNNN-slug.md)`, edited in the commit
   that adds the amending ADR.
2. Keep `Accepted` and rely on `docs/architecture.md` and `AGENTS.md` to point at the newer ADR,
   as was done for ADR-0011 to ADR-0013.
3. Supersede the whole older ADR and restate in the new one the parts that still hold.
4. Edit the older ADR in place with notes such as "this section no longer applies, see
   ADR-NNNN".

Option 2 leaves the older ADR silent: the reader learns about the change only when arriving
through the index. Option 3 copies text that did not change, so the record of why it was decided
splits across two files, and ADRs grow with every change. Option 4 breaks ADR-0001's rule that
accepted text is never rewritten, and notes spread through a body are easy to miss and easy to
leave stale.

## Decision Outcome

Chosen option: 1. This record amends the `Status` and "How to supersede" rules of ADR-0001.

- The statuses are `Accepted`, `Accepted, amended by [ADR-NNNN](NNNN-slug.md)`,
  `Superseded by [ADR-NNNN](NNNN-slug.md)` and `Deprecated`.
- Several amending ADRs are listed in number order, separated by commas:
  `Accepted, amended by [ADR-0011](...), [ADR-0012](...)`, each `(...)` holding the file name.
- To amend, write a new ADR. Its context names the sections or sentences of the older ADR that no
  longer apply and what replaces them, in the form ADR-0011 and ADR-0012 already use: "It
  replaces only the X part of ADR-NNNN; everything else stands." The commit that adds it edits
  the `Status` line of every ADR it amends, and nothing else in them.
- The text of an amended ADR is not changed. Fixing typos, broken links or wrong paths stays
  allowed, as ADR-0001 says.
- When amendments add up to replacing most of an ADR, supersede it instead. A superseded ADR's
  status becomes `Superseded by ...` alone; the superseding ADR accounts for the earlier
  amendments.
- `docs/architecture.md` and `AGENTS.md` point at the amending ADR, as they do for a superseding
  one.

The commit that adds this record also sets the status of every partial replacement made before
the status existed:

| ADR      | Status                                  | Part that no longer applies                                                        |
| -------- | --------------------------------------- | ---------------------------------------------------------------------------------- |
| ADR-0001 | Accepted, amended by ADR-0015           | The list of statuses and the rule that only a supersede edits the status line.     |
| ADR-0003 | Accepted, amended by ADR-0011           | Biome as the TypeScript formatter and linter.                                      |
| ADR-0004 | Accepted, amended by ADR-0013           | The Idempotency-Key on every write; the four sentinel errors (now six).            |
| ADR-0007 | Accepted, amended by ADR-0013           | The idempotency record keeping only a body hash (it stores the canonical request). |
| ADR-0009 | Accepted, amended by ADR-0011, ADR-0012 | Biome; lefthook as the git hook runner.                                            |

In the ADR files each `ADR-NNNN` of a status line is a link.

### Consequences

- Good: the first lines of an ADR tell a reader, or an agent, whether the whole record still
  holds, and which records to read next.
- Good: `grep -l 'amended by' docs/adr` lists every partly replaced record.
- Good: accepted decisions are still never rewritten; the reasons behind the parts that hold
  stay where they were written.
- Bad: the status of an ADR that is amended often gets long, and it does not say which part each
  amendment touches; the reader has to open the amending ADRs, whose context names the part.
- Bad: one more rule for reviewers. Nothing checks that a pull request adding an amending ADR
  also edits the status lines; it is a review discipline, like the rest of ADR-0001.
- Bad: tools that parse MADR statuses into a fixed set would not recognise the new value. None is
  used in this repository.

## More Information

- [ADR-0001](0001-record-architecture-decisions.md): the status and supersede rules this record
  amends.
- [ADR-0011](0011-vite-plus-toolchain-for-typescript.md),
  [ADR-0012](0012-vite-plus-git-hooks.md),
  [ADR-0013](0013-ledger-api-implementation.md): the partial replacements this record gives the
  new status (ADR-0011 lines 17-18, ADR-0012 lines 17-18, ADR-0013 lines 38, 68 and 87).
- MADR, whose status field is free text: https://adr.github.io/madr/
