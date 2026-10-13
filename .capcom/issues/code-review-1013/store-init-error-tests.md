---
title: Store Init and RaiseFormat error branches untested
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`capcom init` runs `store.Init` on a user's repository, and its error branches are untested:
`createDir` finding `.capcom` present and not a directory, `createConfig`'s open, write and
close errors, and `RaiseFormat`'s read error other than not-exist. A stray `.capcom` file is
the realistic way init fails. Severity: Low. These tests come before the guarded
`config.yaml` read and the missing-config format fix, which change `store.go`.

## Acceptance

- With `.capcom` as a regular file, `Init` errors naming `.capcom` and creates nothing.
- With `config.yaml` as a directory, `RaiseFormat` errors and the entry is unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/store/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-09.
