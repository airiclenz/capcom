---
title: list and search --excerpt run one query per row, so they slow down with the store
status: open
type: bug
priority: 3
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:27:17Z
---

`capcom list` and `capcom search --excerpt` issue one or more SQL queries per row, so their cost
grows with the store in round trips rather than in data.

`list` runs five statements per issue: the row itself, then four list queries for its labels,
deps, related links and aliases (`internal/index/index.go:191-226`, called per id from
`internal/index/query.go:325`). `search --excerpt` reads one body per hit
(`internal/cli/search.go:114`). `ready` walks every issue to rank them.

Measured on a store of 12,000 issues: `list` takes 0.52 s, and `search --excerpt` 1.10 s against
0.14 s without excerpts. Neither is slow at today's sizes, but both grow linearly, and a rebuild of
the same store takes 19 s (`index-busy-timeout-scale`).

## Acceptance

- `list` reads its rows and their list fields in a fixed number of queries, whatever the count.
- `search --excerpt` reads the bodies of its hits in one query.
- Go benchmarks `BenchmarkList` and `BenchmarkSearchExcerpt` in `internal/index` (or the package
  that owns each query) run against a generated store of a few thousand issues, so a regression
  is visible with `go test -bench`.
