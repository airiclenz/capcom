---
title: No test for tap or bucket clone, commit and push failures in publish
status: done
type: task
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Nothing in the `cmd/release` tests makes `gh repo clone`, `git rev-list`, `git commit` or
`git push` fail for the Homebrew tap or the Scoop bucket. `publishPackageFile` wraps each of
these failures, and `publish` must stop on them, but the fake `respondChannel` behind
`fakeChannels` has no failure toggle, so none of those error paths is exercised. The
release-convergence path is marked critical. Severity: Medium.

## Acceptance

- `fakeChannels` gains a failure toggle.
- A table test fails clone, rev-list, commit and push in turn, for the tap and for the bucket,
  and asserts that `publish` returns an error naming the repository and the step, and that
  `smoke` does not run.
- Each case fails against a deliberately broken implementation (bite check).
- `GOMEMLIMIT=2GiB go test ./cmd/release/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-01.
