---
title: Ref path#symbol check reads FIFOs and huge files
status: done
type: bug
priority: 2
deps: [guarded-file-reader]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The Ref `path#symbol` check (`checkSymbol` in `internal/knowledge/verify.go`) reads the named
file with `os.Stat`, `os.ReadFile` and `string(data)`, so it reads FIFOs and huge files. It is
reached from `check`, `know verify` and `prime` (through `check.Run`). Severity: Medium.

Trigger: a Ref `blob.bin#x` naming a huge committed file is held in memory twice on every
`prime`. A Ref naming a local FIFO (an untracked local entry; git cannot commit one) blocks
`prime` and `check` forever. Symlink escapes are already refused.

Wanted: read through the guarded reader with the cap as a named constant, map `ErrNotRegular`
and `ErrTooLarge` to Ref Findings ("is not a regular file", "is larger than …"), and search with
`bytes.Contains` instead of `string(data)`.

## Acceptance

- A knowledge test with a FIFO Ref and an oversized Ref asserts a Finding, not a hang;
  `fifo_unix_test.go` and `fifo_windows_test.go` pin this per OS.
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-02(c).
