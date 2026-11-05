---
title: No test runs many capcom processes at once on one store
status: open
type: task
priority: 2
labels: [maturity, testing, concurrency]
plan: 2026-11-05 - 00 - p2-maturity-plan
created: 2026-11-04T18:27:23Z
---

Several agents in one repository means several `capcom` processes at once: in one worktree
(sharing the index under the git directory and the Tree lock) and across `git worktree`s
(sharing the git directory). The CLI tests drive commands in process and never in parallel
(Note `cli-test-harness`), so races between real processes over the SQLite index rebuild,
atomic file replaces (`internal/nodefile`), the lock record and token files
(`internal/lock`) and the store format raise are untested.

Add a stress test that builds the real binary once (`go build -o <tmp>/capcom ./cmd/capcom`
in `TestMain` or a helper, as `cmd/agenteval` builds its build under test) and runs it as
subprocesses with `os/exec`, behind a build tag `stress` or `testing.Short()` skip so the
normal suite does not pay for it:

1. **One worktree:** 16 goroutines each run 50 random commands (`create` with unique ids,
   `log`, `update --priority`, `dep add` between issues they created, `close`, `ready`,
   `prime`, `reindex`, `lock acquire/release` with their own purpose) against one store.
   Afterwards: `capcom check` exits 0; every create that exited 0 has its file; every `log`
   that exited 0 is in the file; `capcom reindex` followed by `list --all --json` equals the
   listing before it (the index matches the files); no process exited with a panic or an
   SQLite `database is locked` error that was not reported as a clean, documented failure.
2. **Several worktrees:** the same against three `git worktree add` checkouts of one
   repository, each with its own Tree lock, asserting the locks never cross worktrees and
   each worktree's index stays consistent with its files.
3. **Lock contention:** 8 processes race `capcom lock acquire --purpose p<n>`; exactly one
   wins, the rest exit 3 and name the holder.

Run it in CI as a separate job on ubuntu-latest and macos-latest
(`go test -tags stress -count=1 -run Stress ./internal/cli`), and record any bug it finds as
a fix with a CHANGELOG entry. On the Raspberry Pi host use 4 goroutines and
`GOMEMLIMIT=2GiB`.

## Why

Many agents at once is the scenario capcom is built for, and the one most likely to corrupt
the index or lose a write; it should be proven by a test, not assumed.

## Acceptance

- The three stress scenarios exist behind the `stress` tag and pass 10 runs in a row locally.
- A CI job runs them on Linux and macOS.
- Any race found is fixed, with a regression test and a CHANGELOG entry.
