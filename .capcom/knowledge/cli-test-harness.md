---
title: CLI tests run through runCLI, hermetic and never parallel
labels: [testing]
refs: [internal/cli/testharness_test.go#func runCLI, docs/adr/0007-tests-use-real-temp-files-sqlite-and-git.md]
---

CLI tests drive every command in process through `runCLI` (or `runCLIWith` for stdin and
environment), in a real temp git repository with a real SQLite index (ADR 0007). They change the
working directory and process state, so they never call `t.Parallel()`.

On the Raspberry Pi 5 host, run one package or one test at a time with `GOMEMLIMIT=2GiB`, for
example `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`, and never `./...` with `-race`.
