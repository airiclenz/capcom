---
title: dep rm and related rm prefix-resolve a dangling entry before exact text
status: done
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`dep rm` and `related rm` resolve the argument by unique prefix before matching a dangling entry
by its exact text. Their doc comment promises that a dep naming no issue at all is removed by its
exact text, but that fallback runs only when resolution returns `ErrNotFound`, so a dangling
entry whose text prefixes a live id resolves to that live Issue instead. Severity: Medium for
`dep rm`, Low for `related rm`.

Trigger: Issue `x` has `deps: [auth]`; `auth` was removed or renamed while `auth-refactor`
exists. `capcom dep rm x auth` resolves to `auth-refactor` and fails "x does not depend on auth".
If x also depends on `auth-refactor`, the command removes that valid dep and keeps the dangling
one. If `auth` prefixes two Issues, it fails as ambiguous. The dangling dep blocks x for good,
and only a hand edit clears it.

Wanted: an entry whose stored text equals the normalised argument is removed, whatever the
argument also resolves to. In `depRemoval` and `relatedRemoval`, test exact text first when the
argument is not an exact id or Alias of a live Issue; otherwise resolve as today, and on
`tree.ErrAmbiguous` fall back to the exact-text test before failing.

## Acceptance

- CLI tests for `dep rm` and `related rm` cover: a dangling entry that prefixes one live Issue; a
  dangling entry that prefixes two; a dangling entry plus a valid dep on the prefixed Issue (only
  the dangling one is removed); and the existing exact-id path, unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'DepRm|RelatedRm'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-06.
