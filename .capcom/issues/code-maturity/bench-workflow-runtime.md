---
title: bench.yml may not finish inside its 360-minute timeout
status: open
type: task
priority: 3
labels: [maturity, ci]
deps: [index-build-quadratic]
created: 2026-11-05T11:02:17Z
---

`.github/workflows/bench.yml` runs every benchmark with `-count=6 -benchtime=3x` on the commit and again on its parent. On the Raspberry Pi 5 host one `-benchtime=1x` pass over N=10000 already costs about 55s for `BenchmarkReindex`, 55s for `BenchmarkColdFirstCommand` and 126s for the recap benchmark (`docs/reviews/benchmarks-2026-11-05.md`). Multiplied by 6 counts, 3 iterations and two sides, the job may come close to its `timeout-minutes: 360`. The workflow has never run on GitHub, so its real runtime on ubuntu-latest is unknown. `command-benchmarks` stays open until that first run.

## Why

A benchmark job that times out reports nothing, so the regression gate it exists for silently stops working. A job that takes hours on every push to `main` also wastes runner minutes.

## Acceptance

- The first `bench.yml` run on GitHub is recorded (its duration, linked from the benchmark baseline doc).
- The job finishes in under 60 minutes, by fixing `index-build-quadratic` and `recap-large-store` or by trimming the counts/benchtime for the slowest benchmarks, and the benchstat comparison still has enough samples for p < 0.05.
