---
title: Elsewhere scan git-failure warnings and file-name filter untested
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The Elsewhere scan promises it never fails, turning every git failure into a warning, but that
contract is pinned only for an unborn HEAD and an unparseable copy. The `Branches`,
`IsAncestor` and `ChangedFiles` failure warnings are untested, and the `issueID` filter that
skips hidden, temporary and non-`.md` files is never exercised. Severity: Low. These tests come
before the branch-blob size cap and the `elsewhere.days: 0` fix, which change the same file.

## Acceptance

- `Scan` at a directory that is not a repository gives one "list branches" warning and no
  entries.
- A corrupt or dangling branch ref gives a per-branch warning while a healthy second branch is
  still reported.
- A side branch adding `.hidden.md`, `notes.txt` and one real Issue reports only the real
  Issue.
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-07.
