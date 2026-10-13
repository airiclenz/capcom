---
title: Move, remove and fmt overwrite other Nodes' files from a stale snapshot
status: open
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

Move, remove and fmt overwrite other Nodes' files from a stale snapshot. Every planner reads
the files it will rewrite at the start, and `tree.Plan.AddRewrite` then writes `Content` with
only a `Stat`, and undo writes `Original` back, both blindly. Nothing serialises writers across
processes, and the deployment is several agent sessions in one clone. Severity: Medium.

Trigger: session A runs `capcom mv auth-fix login-fix`, which reads every Issue, runs its claim
checks, then applies. Meanwhile session B runs `capcom close api-work`, an Issue depending on
`auth-fix`. A then writes its pre-close copy of `api-work` with the renamed dep, and B's close is
lost silently. A rollback after a later failing step writes `Original` over B's change as well.
`--if-match` on A guards only the moved file, and the window grows with the tree.

Wanted (decided 2026-10-13: local compare-and-swap only, no store-wide write lock): each rewrite
is a compare-and-swap. In apply, re-read the file through the guarded reader and fail with a
stale error (wrapping the sentinel `--if-match` already uses, so the CLI maps it to the same
code) when the bytes differ from `Original`, which rolls back the steps already done. In undo,
restore only when the file still equals `Content`, and otherwise report a rollback failure that
names the file. `Format` does the same per file.

## Acceptance

- A tree test changes a file F after its rewrite is planned and before `Apply`, and asserts that
  `Apply` fails with the stale error, F holds the concurrent bytes and earlier steps were rolled
  back.
- A second tree test changes F after the rewrite and before a failing later step, and asserts
  that undo does not overwrite it and the rollback failure names F.
- `GOMEMLIMIT=2GiB go test ./internal/tree/`, then `./internal/nodestore/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-11(a).
