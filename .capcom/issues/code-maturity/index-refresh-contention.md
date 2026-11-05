---
title: Concurrent commands wait out the 5s busy timeout on index refresh
status: done
type: bug
priority: 3
labels: [maturity, performance]
plan: 2026-11-05 - 00 - large-store-performance-plan
created: 2026-11-05T11:02:17Z
closed: 2026-11-05T12:34:19Z
reason: resolved by large-store-performance-plan (archived in 385c441), item 3
---

Every capcom command refreshes the index in one IMMEDIATE transaction (`Index.refresh` in `internal/index/refresh.go`, `dsnOptions` in `internal/index/index.go`), even when no file changed and the refresh writes nothing. IMMEDIATE takes the write lock up front, so concurrent commands on one store queue behind each other. With 16 processes on one store (`TestStress_OneWorktree` at its default worker count), some commands wait the full 5s busy timeout on "begin refresh" and fail "the index is busy". The stress test accepts that failure as documented when the process ran at least 4s (`busyWaitFloor` in `internal/cli/stress_test.go`), so it does not fail on it.

## Why

Several agents in one worktree, or one agent running commands in parallel, is the case capcom is built for. A read-only command such as `show` or `ready` failing busy because other reads hold the write lock is a spurious failure the agent has to retry.

## Acceptance

- A refresh that finds no changed file does not hold the write lock (for example: check in a deferred transaction, upgrade only when there is something to write).
- A stress test pins it: on a quiescent store, with one process holding the index write lock past the busy timeout, parallel `ready` and `prime` processes all exit 0. The mixed `TestStress_OneWorktree` workload keeps accepting a busy failure after the wait, since its reads usually find files other workers just wrote (decided 2026-11-05: a clean refresh takes no write lock; a read-only command may still fail busy after the wait when files changed).
- Two refreshes that both have changes to write still never deadlock (a test in `internal/index` pins it).
