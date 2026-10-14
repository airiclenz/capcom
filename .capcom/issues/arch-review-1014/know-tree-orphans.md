---
title: know tree drops the children of an unparseable Note
status: open
type: bug
priority: 2
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
---

`capcom know tree` leaves out a Note whose parent Note is not indexed (its file does not parse,
or it is left out as a repeated id): the Note is valid and indexed, but no line of the outline
shows it, so it is invisible in the one view meant to list every Note. `capcom list --tree`
handles the same case for issues by printing such an issue at the root.

Wanted (ratified 2026-10-14): a Note whose parent is not shown prints at the root, as issues do,
including inside a background or retired subtree, where know tree's existing background rules
keep applying.

## Acceptance

- In `capcom know tree`, a Note below an unparseable or left-out Note prints at the root; a
  runCLI test covers it.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/` passes.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, Smaller locality fixes (Visible-subtree outline).
