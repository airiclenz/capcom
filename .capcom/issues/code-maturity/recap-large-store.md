---
title: recap takes two minutes over 30 days of a 10000-issue store
status: open
type: task
priority: 2
labels: [maturity, performance]
created: 2026-11-05T10:50:43Z
---

`capcom recap` over a 30-day window takes 3.59 s on a store of 1000 issues and 125.7 s on one of
10000 (`BenchmarkRecap` in the benchmark baseline, `docs/reviews/benchmarks-2026-11-05.md`,
Raspberry Pi 5 host). Both generated stores have the same history shape: about 500 commits
touching `.capcom/`, part of them inside the window.

The likely cause: `internal/recap` reads every changed file's copy at each end of the window
through `load`, which calls `gitx.ShowFile`, and `ShowFile` starts two git processes per file
(`git cat-file -s` and `git cat-file blob`). With thousands of files changed in the window that
is thousands of process starts, each costly on the Pi. One `git cat-file --batch` process per
revision, fed every object name, would read the same blobs in one pass.

## Why

Recap is the command an agent runs on returning to a repository after time away, which is when
the window holds the most change; two minutes on a large store makes it unusable.

## Acceptance

- `BenchmarkRecap` at N=10000 runs under 10 s on the Raspberry Pi 5 host
  (`GOMEMLIMIT=2GiB go test -run='^$' -bench=Recap -benchtime=1x ./internal/cli`), and the new
  number is added to the benchmark baseline doc.
- The size limit on a file's copy (`maxFileSize`, `gitx.ErrBlobTooLarge`) and the warning for a
  copy that cannot be read still hold, each pinned by a test.
