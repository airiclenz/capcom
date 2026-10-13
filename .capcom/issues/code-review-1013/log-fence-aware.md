---
title: Log section lookup ignores fenced code blocks
status: done
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The Log section is found without regard to fenced code blocks. `appendLogEntry` takes the first
`## Log` line and the merge's `parseLog` does the same, while `rawfm`'s conflict scan already
skips fences. Severity: Low.

Trigger: an Issue body quotes the Log format inside a code fence above the real `## Log`
section. `capcom log` and `reopen` then insert the entry after the example, in the description.
On a merge, `parseLog` takes the same fake section and falls back to a plain text merge, so
concurrent Log appends on two branches conflict.

Wanted: one fence-aware line scanner, shared by the Log append and the merge, that tracks
```` ``` ```` and `~~~` fences open and closed, reusing `rawfm`'s approach.

## Acceptance

- `mutate_test.go` and `merge_test.go` cases with a fenced `## Log` example before the real
  section: the append lands in the real Log, and the merge of two appends succeeds entry by
  entry.
- `GOMEMLIMIT=2GiB go test ./internal/issue/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-22.
