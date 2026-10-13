---
title: Index reads after Open do not come from one snapshot
status: done
type: bug
priority: 3
deps: [refresh-race-tests]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Index reads after `Open` do not come from one snapshot. `List` reads each Issue with five
statements (the row, then labels, deps, related and Aliases), and only the refresh is a
transaction. Severity: Low (transient and read-only; a retry is consistent).

Trigger: agent A runs `capcom ready`, which lists every Issue in about 5×N statements, while
agent B's command commits a refresh that removes or renames an Issue partway through. A's command
fails as a whole with `id "x" not found`, including in `close` after its write has already
succeeded. Without a failure, an Entry can mix two versions, such as the category of one and the
deps of the other.

Wanted: each command's index queries read from one snapshot (a deferred read transaction on a
single connection, or set-based statements fetching every Entry's rows), and an id that vanished
is skipped instead of failing the command.

## Acceptance

- An index test lists Issues, deletes one Issue's rows through a second connection
  mid-iteration (through a seam), and asserts that `List` returns a consistent set without error.
- `GOMEMLIMIT=2GiB go test ./internal/index/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-20.
