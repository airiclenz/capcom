# Command benchmark baseline — 2026-11-05

The first numbers from the whole-command benchmarks in `internal/cli/bench_test.go`. Each
benchmark runs one capcom command in process on a generated store of N issues with 200 Notes,
nested parents to depth 4, acyclic deps, 0 to 5 Log entries per issue, about 500 commits of
history touching `.capcom/` and 20 recent branches. `.github/workflows/bench.yml` compares every
push to `main` against its parent; this page is the absolute reference those relative checks lack.

## How they were measured

- **Command:** `GOMEMLIMIT=2GiB go test -run='^$' -bench=. -count=3 -timeout=0 ./internal/cli`
  (`-timeout=0` because the run takes about 31 minutes, past `go test`'s default 10-minute limit)
- **Commit:** `99fc3d9`
- **Run:** 2026-11-05, 10:18 to 10:49 UTC (1866 s)
- **Host:** Raspberry Pi 5 Model B Rev 1.1, 4 Cortex-A76 cores, 7.9 GiB RAM; Linux 6.18.59
  arm64; go1.26.10; git 2.47.3; `modernc.org/sqlite` v1.44.3. The stores live in `b.TempDir()`
  under `/tmp`, which is tmpfs on this host, so disk speed plays no part.
- **Values:** the mean time per command over the three counts. The three counts of every row
  lie within 5% of each other. A row slower than 1 s at N = 10000 names the follow-up issue
  filed under `code-maturity` to bring it down.

## Results

| Command | Benchmark | N = 1000 | N = 10000 | Follow-up issue |
| --- | --- | ---: | ---: | --- |
| `prime` (warm index) | `BenchmarkPrime` | 0.47 s | 2.51 s | `warm-reads-large-store` |
| `ready` | `BenchmarkReady` | 0.37 s | 1.62 s | `warm-reads-large-store` |
| `next` | `BenchmarkNext` | 0.18 s | 1.25 s | `warm-reads-large-store` |
| `list --all --json` | `BenchmarkListAllJSON` | 0.37 s | 1.59 s | `warm-reads-large-store` |
| `search <word>` | `BenchmarkSearch` | 0.041 s | 0.35 s | — |
| `check` | `BenchmarkCheck` | 0.094 s | 0.81 s | — |
| `recap --since <30 days ago>` | `BenchmarkRecap` | 3.59 s | 125.74 s | `recap-large-store` |
| `reindex` | `BenchmarkReindex` | 1.08 s | 53.95 s | `index-build-quadratic` |
| `list` after deleting the index | `BenchmarkColdFirstCommand` | 1.42 s | 54.51 s | `index-build-quadratic` |
| `log <id> <text>` with its index update | `BenchmarkLog` | 0.042 s | 0.33 s | — |

## What the numbers say

- **Building the index is quadratic.** `reindex` and the cold first command cost fifty times
  more at ten times the issues. Every file's rows are dropped before they are written, and the
  drop from the FTS5 table scans it whole, since its `path` column is unindexed. Issue
  `index-build-quadratic`.
- **Recap pays per changed file.** It starts two git processes for every file copy it reads at
  each end of the window, so its cost follows how much changed, not the issue count. Issue
  `recap-large-store`.
- **The warm read commands carry a cost beyond the index.** `search` and `log` refresh the same
  index in about 0.35 s at N = 10000, and `check` parses every file in 0.81 s, yet `ready`, `next`,
  `list` and `prime` take 1.25 to 2.51 s. All four add the report of issue changes on other
  branches, which the generated store's 20 branches feed. Issue `warm-reads-large-store` profiles
  them first.
- **Search, check and log scale linearly** and stay under 1 s at N = 10000.

N = 50000 (`CAPCOM_BENCH_LARGE=1`) was not run: at the costs above its index build and recap
alone would take hours on this host.

## 2026-11-05 — index build after `index-build-quadratic`

A refresh now drops a file's rows only when the index holds a files row for it, so a cold build
and a `reindex` no longer scan the whole FTS5 table once per file. The baseline rows above stand
as measured.

- **Command:** `GOMEMLIMIT=2GiB go test -run='^$' -bench='Reindex|Cold' -count=3 -timeout=0 ./internal/cli`
- **Commit:** `6d1db27` with the change `perf(index): forget a file's rows only when the index holds them`
- **Run:** 2026-11-05, 11:49 to 11:51 UTC (103 s), on the same host and toolchain as the baseline.
- **Values:** the mean time per command over the three counts. The counts spread wider than the
  baseline's: one count of each N = 10000 row ran about 1.5 s slower than the other two, and the
  N = 1000 cold first command took 1.84 to 2.10 s here against 0.83 s in a single
  `-benchtime=1x` run just before.

| Command | Benchmark | N = 1000 | N = 10000 | N = 10000 / N = 1000 |
| --- | --- | ---: | ---: | ---: |
| `reindex` | `BenchmarkReindex` | 0.41 s | 4.33 s | 10.6× (9.3× without the slow count) |
| `list` after deleting the index | `BenchmarkColdFirstCommand` | 1.94 s | 5.34 s | 2.8× |

Both stay under 10 s at N = 10000, down from about 54 s. `reindex` costs about 0.4 ms per issue at
either size, so the build now grows linearly with the store.

## 2026-11-05 — recap after `recap-large-store`

Recap now reads every file copy it needs through one `git cat-file --batch` process instead of two
git processes per copy. The baseline rows above stand as measured.

- **Command:** `GOMEMLIMIT=2GiB go test -run='^$' -bench=Recap -benchtime=1x -count=3 -timeout=0 ./internal/cli`
- **Commit:** `583586d` with the change `perf(recap): read every file copy through one git cat-file process`
- **Run:** 2026-11-05, 12:29 to 12:30 UTC (23 s), on the same host and toolchain as the baseline.
- **Values:** the mean time per command over the three counts. One N = 10000 count took 1.44 s
  against 0.81 s and 0.84 s for the other two.

| Command | Benchmark | N = 1000 | N = 10000 | N = 10000 / N = 1000 |
| --- | --- | ---: | ---: | ---: |
| `recap --since <30 days ago>` | `BenchmarkRecap` | 0.097 s | 1.03 s | 10.6× (8.5× without the slow count) |

N = 10000 runs in about 1 s, down from about 126 s and well under the 10 s goal.

## 2026-11-05 — one count of every benchmark, for `bench.yml`'s runtime

With both fixes in, one count of every benchmark at the flags `bench.yml` uses runs in 235 s on
this host. Each side of the workflow runs six counts, about 24 minutes here, and the job runs two
sides, about 47 minutes. A side exceeds `go test`'s default 10-minute limit, so `bench.yml` now
passes `-timeout=30m` and caps the job at 70 minutes instead of 360. About half of each count
builds the generated stores: every benchmark builds its own store at each size.

- **Command:** `GOMEMLIMIT=2GiB go test -run='^$' -bench=. -count=1 -benchtime=3x -timeout=0 ./internal/cli`
- **Commit:** `4c359e4`
- **Run:** 2026-11-05, 235 s, on the same host and toolchain as the baseline.
- **Values:** one count of three timed runs per benchmark, so they are noisier than the rows above.

| Benchmark | N = 1000 | N = 10000 |
| --- | ---: | ---: |
| `BenchmarkPrime` | 0.46 s | 3.33 s |
| `BenchmarkReady` | 0.36 s | 1.56 s |
| `BenchmarkNext` | 0.16 s | 1.20 s |
| `BenchmarkListAllJSON` | 0.37 s | 1.81 s |
| `BenchmarkSearch` | 0.036 s | 0.35 s |
| `BenchmarkCheck` | 0.11 s | 1.11 s |
| `BenchmarkRecap` | 0.096 s | 0.83 s |
| `BenchmarkReindex` | 0.36 s | 4.25 s |
| `BenchmarkColdFirstCommand` | 0.76 s | 5.32 s |
| `BenchmarkLog` | 0.069 s | 0.49 s |
