---
title: Briefing ready, doing, check and Elsewhere failure branches untested
status: open
type: task
priority: 3
deps: [briefing-panic-test]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

`briefing.Compose` documents a failure order ("store, index, ready, doing, locks, check, then
Elsewhere warnings") and a partial-Work rule, but only the index and locks failures are pinned
by the ordered-failure test. The index Close failure, the `SectionReady`, `SectionDoing` and
`SectionCheck` failures and the one `SectionElsewhere` Failure per Elsewhere warning are
untested, so the order and the "other sections survive" rule can drift unnoticed.
Severity: Low.

## Acceptance

- The ordered-failure test is extended with a `check.Run` error and an Elsewhere warning (for
  example a store in a repository with an unreadable branch ref), and with ready and doing
  failures through the seam added for `briefing-panic-test`.
- It asserts the section order of the Failures, `Health == nil` on a check failure, and that
  the other sections survive.
- `GOMEMLIMIT=2GiB go test ./internal/briefing/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-03.
