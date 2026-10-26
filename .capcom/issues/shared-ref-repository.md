---
title: Three call sites build the same knowledge.Repository by hand
status: open
type: chore
priority: 3
created: 2026-10-26T10:49:10Z
---

`knowledge.Verify` takes a `knowledge.Repository` (working-tree root, git dir, common git dir,
ignore lookup). Three call sites build the identical literal from the store's fields plus
`gitx.IgnoredPaths`:

- `internal/cli/know_add.go` (`checkNewRefs`, used by `know add` and `know update`)
- `internal/cli/know_verify.go` (`know verify`)
- `internal/check/notes.go` (`checkRefs`, used by `capcom check`)

Why: the three must agree on what a clone would lack. A new field on `Repository`, or a change
to how ignored paths are looked up, has to land in all three. Missing one would make `check`,
`know verify` and `know add` disagree about the same Ref. Found by the verifier of the
field-eval-ergonomics plan, item 11.

## Acceptance

- The `knowledge.Repository` for a store is built in one place, and all three call sites use it.
- No behaviour change: the existing `internal/knowledge`, `internal/check` and `internal/cli`
  tests pass unchanged.
