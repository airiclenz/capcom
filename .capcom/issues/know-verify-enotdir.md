---
title: know verify aborts on a ref below a file
status: open
type: bug
priority: 2
created: 2026-10-11T14:13:10Z
---

`capcom know verify` aborts with an error (exit 1, no list) on a Ref like
`README.md/sub` whose parent is a file. `checkRef` in
`internal/knowledge/verify.go:118` treats only `fs.ErrNotExist` from
`filepath.EvalSymlinks` as a missing path (`internal/knowledge/verify.go:119`),
so the `ENOTDIR` error is returned as a hard failure and every other finding is
hidden too.

Found implementing plan `2026-10-11 - 01 - capcom-v1-knowledge-lock-briefing-plan`,
item 11 (Knowledge: Notes and `capcom know`).

## Acceptance

- A Ref whose parent path is a file is listed as a broken Ref like a missing path,
  and the other findings still print.
- A test covers a `<file>/sub` Ref.
