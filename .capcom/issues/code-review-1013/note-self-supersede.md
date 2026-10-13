---
title: Renaming a Note to its own successor makes it invalid
status: open
type: bug
priority: 3
deps: [reference-rewrite-engine]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

Renaming a Note to its own successor makes it superseded by itself. `knowledge.PlanMove` changes
only the moved Note's Aliases and skips the Note itself when it rewrites successors, and
`nodestore.Move` never validates the planned content, although `knowledge` validation refuses a
Note that "names the note itself". The index then leaves the invalid Note out. Severity: Low.

Trigger: Note `auth-v1` has `superseded-by: [auth]`, and no Note `auth` exists.
`capcom know mv auth-v1 --id auth` writes `auth.md` with `superseded-by: [auth]`, and the Note
drops out of `know tree`, search and the AGENTS.md block.

Wanted: the move either drops the new id from the moved Note's own `superseded-by` (as
`renameSuccessors` drops duplicates) or refuses with an `*Error` citing the `superseded-by` line;
`nodestore.Move` running `Validate` on the planned content is the stronger guard. With
`reference-rewrite-engine`'s single rename op over every reference field, the moved Node's own
included, this cannot happen.

## Acceptance

- A `knowledge` move test "rename to own successor" either removes the entry or refuses, and
  leaves the Note valid.
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-23.
