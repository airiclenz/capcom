---
title: Index refresh "changed under the walk" branches untested
status: open
type: task
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

Index refresh is a named critical path, but its "changed under the walk" branches are
untested. Only a file deleted before the walk is covered
(`TestRefresh_FileDeleted_DropsItsRows`). Untested: a listed file that vanishes between the
walk and `ReadNodeFile` (the not-exist branch after `forget`), a file whose `Lstat` reports
not-exist in the index walk, and `rewarn` on corrupt stored `files.problems` JSON. The index
walk has the lowest coverage in its scope. Severity: Medium. These tests come before the
changes to `refresh.go` for left-out name claims and snapshot reads.

## Acceptance

- A test deletes a listed file between the walk and the read (through a `commitTx`-style seam
  or a new read hook) and asserts that `Refresh` succeeds, the file's rows are gone and it is
  not in `Parsed`.
- A test writes bad JSON into `files.problems` and asserts that `Refresh` returns an error
  naming the path.
- `GOMEMLIMIT=2GiB go test ./internal/index/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-04.
