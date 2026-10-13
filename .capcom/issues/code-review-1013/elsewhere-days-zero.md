---
title: elsewhere.days 0 still scans future-dated branches
status: done
type: bug
priority: 3
deps: [elsewhere-failure-tests]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`elsewhere.days: 0` still scans branches dated in the future. The `Options.Days` doc says 0
"ignores every branch", but the scan keeps a branch whose tip is after `Now - Days*day`, and a
tip whose committer date is in the future (clock skew, or a hostile remote-tracking branch) still
passes. The result is extra Elsewhere entries and scan work; `--no-elsewhere` still disables the
scan fully. Severity: Low.

Wanted: `Scan` returns early when `Days == 0`, before any git call.

## Acceptance

- An `elsewhere_test.go` case with `Days: 0` and a future-dated side branch (a fixed committer
  date in the environment) gets an empty report and runs no git.
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-18.
