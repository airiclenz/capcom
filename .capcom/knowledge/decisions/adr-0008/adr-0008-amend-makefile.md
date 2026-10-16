---
title: "ADR 0008 amendment of 2026-10-16: a root Makefile for local builds, never on the release path"
---

Amended 2026-10-16: the repository root has a `Makefile` for local development. `make build` runs
the same static build as `dist`, `CGO_ENABLED=0 go build -trimpath`, into `./capcom`; `make vet`
and `make test` run `go vet` and `go test` over `PKG` (`./...` by default); `make clean` removes the
binary. `BUILDFLAGS` and `LDFLAGS` pass extra flags through and are empty by default.

It is a convenience, not a dependency. Releases still go only through `go run ./cmd/release`,
CI still calls `go` directly, and every target stays a plain `go` command, so a machine without
`make`, such as stock Windows, loses nothing. The rejected option stands: no bash scripts, no
`-ldflags` version stamping, and the release pipeline never calls `make`.
