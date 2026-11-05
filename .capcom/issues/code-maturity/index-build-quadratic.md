---
title: Building the index from nothing grows with the square of the store
status: done
type: task
priority: 2
labels: [maturity, performance]
plan: 2026-11-05 - 00 - large-store-performance-plan
created: 2026-11-05T10:50:43Z
closed: 2026-11-05T12:34:19Z
reason: resolved by large-store-performance-plan (archived in 385c441), item 2
---

Building the index from nothing takes time that grows with the square of the store size. The
benchmark baseline in `docs/reviews/benchmarks-2026-11-05.md` measured, on the Raspberry Pi 5
host, `capcom reindex` at 1.08 s with 1000 issues and 53.95 s with 10000 (`BenchmarkReindex`),
and the first `capcom list` after deleting the index at 1.42 s and 54.51 s
(`BenchmarkColdFirstCommand`): ten times the issues costs fifty times the time.

The likely cause: `refresher.indexFile` in `internal/index/refresh.go` calls `refresher.forget`
before it writes a file's rows, and `forget` runs `DELETE FROM <table> WHERE path = ?` on every
table in `rowTables`, `fts` included. `fts` is an FTS5 table whose `path` column is `UNINDEXED`
(`internal/index/schema.go`), so each delete scans the whole full-text table: one scan per
indexed file. A rebuild that has just dropped every table has nothing to forget at all.

## Why

The cold build is what an agent pays after `git pull`, a fresh clone or a deleted index: close
to a minute at 10000 issues is long enough for an agent to give up on the command or time out.

## Acceptance

- `BenchmarkReindex` and `BenchmarkColdFirstCommand` at N=10000 grow at most about ten times
  over N=1000 (`GOMEMLIMIT=2GiB go test -run='^$' -bench='Reindex|Cold' -benchtime=1x ./internal/cli`).
- Both stay under 10 s at N=10000 on the Raspberry Pi 5 host; the new numbers are added to the
  benchmark baseline doc.
- A test pins that a file changed after a refresh still leaves exactly one `fts` row for it.
