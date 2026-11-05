---
title: prime, ready, next and list take over a second on a 10000-issue store
status: open
type: task
priority: 2
labels: [maturity, performance]
created: 2026-11-05T10:50:43Z
---

With a warm index, the read commands an agent runs most take over a second on a store of 10000
issues. The benchmark baseline (`docs/reviews/benchmarks-2026-11-05.md`, Raspberry Pi 5 host)
measured at N=1000 and N=10000:

- `capcom prime`: 0.47 s and 2.51 s (`BenchmarkPrime`)
- `capcom ready`: 0.37 s and 1.62 s (`BenchmarkReady`)
- `capcom next`: 0.18 s and 1.25 s (`BenchmarkNext`)
- `capcom list --all --json`: 0.37 s and 1.59 s (`BenchmarkListAllJSON`)

These are slower than `capcom check` (0.81 s at N=10000), which parses every file without the
index, and far slower than `capcom search` (0.35 s) and `capcom log` (0.33 s), which refresh the
same index. So the cost is not the index refresh. A lead to confirm first: all four slow
commands add the report of issue changes on other branches (`addElsewhere`, `internal/elsewhere`
`Scan`), and the generated store has 20 recent branches; `search`, `check` and `log` do not.
Profile before changing anything (`go test -bench=Ready -cpuprofile`), and split this issue
if the commands turn out to have different causes.

## Why

`prime`, `ready` and `next` open nearly every agent session; a second or more on each, before
any work starts, is what an adopter with a large repository notices first.

## Acceptance

- A CPU profile of `BenchmarkReady` at N=10000 names where the time goes, recorded in a Log entry.
- `ready`, `next` and `list --all --json` run under 1 s, and `prime` under 1.5 s, at N=10000 on
  the Raspberry Pi 5 host (`GOMEMLIMIT=2GiB go test -run='^$' -bench='Prime|Ready|Next|ListAllJSON' -benchtime=1x ./internal/cli`);
  the new numbers are added to the benchmark baseline doc.
