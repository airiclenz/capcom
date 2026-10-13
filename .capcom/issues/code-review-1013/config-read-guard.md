---
title: config.yaml read through symlinks with no size cap
status: done
type: bug
priority: 2
deps: [guarded-file-reader, store-init-error-tests]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`config.yaml` is read through symlinks with no size cap, in `loadConfig` and in `RaiseFormat`.
Severity: Medium (security).

Trigger: `.capcom/config.yaml -> /dev/zero` makes every command hang or exhaust memory, because
`store.Open` reaches `loadConfig` first. A link to a sensitive file (`~/.ssh/...`) gets parsed,
and the strict-parse errors that cite a line can echo fragments of it into output an agent
relays.

Wanted: read `config.yaml` through the guarded reader and return a path-naming error for a
non-regular or oversized file; a bad config fails the command by design. `RaiseFormat`'s atomic
rename replaces the link itself, so there is no write-through.

## Acceptance

- A store test with `config.yaml` as a symlink to a FIFO and to a file over 1 MiB asserts that
  `Open` fails promptly with an error naming `.capcom/config.yaml`.
- `GOMEMLIMIT=2GiB go test ./internal/store/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-02(b).
