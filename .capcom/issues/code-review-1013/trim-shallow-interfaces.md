---
title: Unexport index, agents and tree symbols used only by tests
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Several exported symbols are used only by tests or by their own package, which widens the
interfaces for nothing. Strength: speculative.

- `index.Refresh`, `index.Rebuild`, `index.WithClock` and the `Report` fields are used only by
  `Open` and by tests.
- `agents.Inspect`, `Write`, `Render`, `TopNotes` and `Path` are used outside the package only by
  a CLI test; production uses only `Sync`, `FileName`, `Error` and `State`.
- `tree.(*Tree).Dir` has no caller outside tests.
- `internal/edit` is a mostly shallow grab bag: `Labels` is reused as a generic list add and
  remove by `know update`, and `ETag` moves to `nodefile` with `nodefile-package`.

Wanted: unexport or delete them, moving test helpers to `export_test.go` (base rule "delete dead
code").

## Acceptance

- Each listed symbol is unexported or deleted, or kept with a named production caller outside its
  package.
- `go vet ./...` is clean, and `GOMEMLIMIT=2GiB go test` passes for `./internal/index/`,
  `./internal/agents/`, `./internal/tree/`, `./internal/edit/` and `./internal/cli/`, one at a
  time.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-15.
