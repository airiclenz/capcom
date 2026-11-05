---
title: The stress tests leave a built capcom binary in the temp dir
status: open
type: bug
priority: 3
labels: [maturity]
plan: 2026-11-05 - 00 - large-store-performance-plan
created: 2026-11-05T10:54:01Z
---

Each `go test -tags stress` run of `internal/cli` builds capcom once into a fresh `os.MkdirTemp("", "capcom-stress-")` directory (`stressBinary` in `internal/cli/stress_test.go:86`) and never removes it, since the package may have no `TestMain` to clean it up. One `capcom-stress-*` directory holding a capcom binary is left in the system temp dir per run.

## Why

On the Raspberry Pi 5 host /tmp is tmpfs, so each local stress run keeps a binary in memory until the next reboot; repeated runs pile up.

## Acceptance

- After `go test -tags stress -run Stress ./internal/cli` finishes, no `capcom-stress-*` directory it created remains in the system temp dir.
