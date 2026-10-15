---
title: know tree drops the children of an unparseable Note
status: done
type: bug
priority: 2
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
closed: 2026-10-15T07:52:56Z
reason: "Plan \"2026-10-15 - 00 - pre-release-fixes-plan\" (9ec93fc) item 7, per the ratified Orphans scope call (user, 2026-10-15): a Note below a repeated-id or invalid-id parent is left out with a warning by ADR 0002 (now pinned by tests and stated in the ADR), and a Note below an unparseable parent prints at the root since b1ff6b8."
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

## Log

- 2026-10-14 Airic Lenz: Plan "2026-10-14 - 00 - architecture-review-fixes-plan" (b1ff6b8) landed: know tree prints a Note below an unparseable or invalid-frontmatter Note at the root, also inside background or retired subtrees; left open: a Note below a repeated-id or invalid-id parent is still left out with a warning.
