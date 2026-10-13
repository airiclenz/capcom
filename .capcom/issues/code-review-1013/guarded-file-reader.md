---
title: One guarded reader for repo-controlled files (regular file, size cap, no follow)
status: open
type: bug
priority: 1
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

Repository-controlled files are read with no regular-file check and no size cap. The
2026-10-12 audit fixed this for node files only, in `tree.ReadNodeFile`; every other
repository-controlled read (`AGENTS.md`, `config.yaml`, Ref symbol files) still uses a plain
`os.ReadFile`. The attacker is a hostile repository or branch: crafted symlinks, FIFOs, device
files and huge files. Severity: High overall. This issue adds the one shared reader that the
per-site fixes route through.

`ReadNodeFile` itself has a Low check-then-open race: a same-user process that swaps the entry
between the `Lstat` and `os.Open` gets a swapped-in symlink followed (the post-open `Stat`
describes the target) or blocks `Open` on a swapped-in FIFO, although its doc comment says a
symlink is not followed and a swapped file is refused.

Wanted: one guarded reader beside `ReadNodeFile` (for example
`tree.ReadRegularFile(path string, limit int64) ([]byte, error)`) returning the existing
sentinels `tree.ErrNotRegular` and `tree.ErrTooLarge`. It `Lstat`s and refuses anything not
regular; opens with `O_NOFOLLOW|O_NONBLOCK` off Windows, then clears `O_NONBLOCK`; `Stat`s the
open handle and refuses it unless `os.SameFile` holds and the mode is regular; and reads
through `io.LimitReader(limit+1)`, refusing anything over the limit. `ReadNodeFile` becomes a
thin wrapper over it, so its doc comment becomes true.

## Acceptance

- The guarded reader refuses a symlink, a FIFO (unix) and a file over the limit with
  `ErrNotRegular` or `ErrTooLarge`, and reads a regular file within the limit.
- A unix tree test swaps a regular file for a symlink through a test seam between the `Lstat`
  and the open, and asserts `ErrNotRegular`.
- `ReadNodeFile` reads through the guarded reader; its existing tests still pass.
- `GOMEMLIMIT=2GiB go test ./internal/tree/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-02 (shared fix and (e)).
