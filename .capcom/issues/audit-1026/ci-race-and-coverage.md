---
title: CI runs no race detector and no coverage check
status: open
type: chore
priority: 2
created: 2026-10-26T20:06:45Z
---

CI never runs the race detector or a coverage check, and the Makefile test target lacks
`-count=1`.

    .github/workflows/ci.yml:24  go vet ./...
    .github/workflows/ci.yml:26  go test -count=1 ./...
    Makefile:20                  go test $(PKG)

The three-OS matrix is good, but nothing runs `-race` and nothing gates coverage. The packages
with real concurrency (the lock guard and heartbeat, the index refresh, the UI's polling) are
never race-checked on any OS. The Makefile target reuses cached results, so a local `make test`
can pass on a cached run of a test whose outcome depends on files or environment outside its
inputs, where CI's `-count=1` would rerun it.

AGENTS.md rules out `-race` on the Raspberry Pi host, but Linux and macOS CI can run it.

## Acceptance

- CI runs `go test -race` on at least the concurrency-relevant packages, on Linux and macOS.
- The Makefile test target passes `-count=1`.
- A coverage floor is recorded, or a decision Note says why not.
