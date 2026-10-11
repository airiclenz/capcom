---
title: Write verbs hide a duplicate-id warning when the file has another index warning
status: open
type: bug
priority: 3
created: 2026-10-11T20:50:57Z
---

`create`, `mv` and `know add/mv/rm` report non-blocking tree problems as warnings, but drop one
whenever the command's index refresh already printed a warning naming the same file
(`triageTreeProblems` in internal/cli/errors.go, following the plan 2026-10-11 - 05 regression
guard "never printed twice"). The dedup keys on the file alone, not on what is wrong with it.

So when file A has an index warning for one reason (say, an unknown status) and also duplicates
the id of file B, the user sees only the index warning; the duplicate-id problem is silently
dropped. It still shows in `capcom check`, so nothing is lost for good, but the write verb's output
hides a second, unrelated defect in A.

## Acceptance

- A tree problem is dropped only when an index warning names the same file for the same kind of
  problem; a different problem on the same file is still reported, once.
- Test: file A with an index-level warning and a duplicate id of B; `create` of an unrelated id
  warns about both, each exactly once, and exits 0.
- The existing no-double-warning tests (items 4 and 5) still pass.
