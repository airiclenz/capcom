---
title: Move the foreign-lock write decision out of internal/cli into internal/lock
status: open
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
---

Whether a CLI write warns of another session's Tree lock (ADR 0017: the holder's own process
chain renews and stays silent, writes of only the purpose Issue stay silent, a stale lock is
reported stale) is decided in `internal/cli/root.go` (`lockWarning`, `renewHolderLock`,
`writesOnly`). Move the decision to `internal/lock`, with direct unit tests; cli keeps the
warning texts. Item 4 of the plan.

## Why

The lock policy belongs with the Tree lock, where another front end can share it.

## Acceptance

- `internal/lock` decides the warning with direct tests; `internal/cli` defines neither renewHolderLock nor writesOnly
- Every lock warning text stays byte-identical
