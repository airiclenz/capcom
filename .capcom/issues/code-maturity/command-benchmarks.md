---
title: No benchmark measures whole commands on a large store with git history
status: open
type: task
priority: 2
labels: [maturity, performance]
plan: 2026-11-05 - 00 - p2-maturity-plan
created: 2026-11-04T18:27:23Z
---

The only benchmarks are `BenchmarkList` and `BenchmarkSearchExcerpt` in
`internal/index/bench_test.go` (3000 synthetic issues, index only) and `BenchmarkRenderTable`
in `internal/ui/markdown`. Nothing measures what an agent waits for: a whole command,
including reading files, rebuilding a stale index and scanning git. `capcom prime` scans other
recent branches (`internal/elsewhere.Scan`, `elsewhere.days` in config) and `capcom recap`
walks history, so they are the likeliest to slow down in a big, old repository.

Add `internal/cli/bench_test.go` with a generator that builds, once per benchmark run in
`b.TempDir()`, a git repository with a capcom store of N issues (N = 1000, 10000, 50000 as
sub-benchmarks), nested parents up to depth 4, random deps (acyclic), 0 to 5 Log entries
each, 200 Notes, about 500 commits of history touching `.capcom/`, and 20 branches with recent
tips that each change a few issues. Benchmark through `runCLI`:

- `prime` (warm index), `ready`, `next`, `list --all --json`, `search <word>`, `check`,
  `recap --since <30 days ago>`;
- `reindex` and a cold first command after deleting the index (the cost an agent pays after
  `git pull`);
- one write (`log`) including its index update.

Report with `b.ReportMetric` per issue where useful. Skip N=50000 unless
`CAPCOM_BENCH_LARGE=1` is set, so a default `go test -bench` stays under a few minutes and
within `GOMEMLIMIT=2GiB` on the Raspberry Pi host.

Track regressions: add a CI job (ubuntu-latest, on push to `main` only) that runs the
benchmarks with `-count=6 -benchtime=3x` on the commit and on its parent and compares them
with `golang.org/x/perf/cmd/benchstat` at a pinned version, failing when any command is more
than 25% slower with p < 0.05. Record today's numbers in `docs/reviews/benchmarks-<date>.md`
as the baseline, and list there any command that takes more than 1s at N=10000 as a follow-up
issue.

## Why

"Fast enough on capcom's own 100 issues" says nothing about a monorepo with years of issues;
numbers at scale are what an adopter with a big repository asks for first.

## Acceptance

- `go test -run='^$' -bench=. ./internal/cli` runs the command benchmarks at N=1000 and 10000; `CAPCOM_BENCH_LARGE=1` adds 50000.
- A CI job compares benchmarks against the parent commit with benchstat and fails on a significant regression over 25%.
- `docs/reviews/benchmarks-<date>.md` records the baseline; every command slower than 1s at N=10000 has a filed follow-up issue.
