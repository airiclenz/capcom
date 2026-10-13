---
title: Move init's settings and .gitattributes editors to internal/hooks
status: open
type: task
priority: 3
deps: [nodefile-package]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

About 300 lines of pure file-editing logic sit in `internal/cli/init.go`: the agent-settings hook
groups, `decodeObject`/`encodeObject` keeping JSON member order, and adding a `.gitattributes`
line while keeping CRLF. They are tested only end to end through `init`. Strength: worth
exploring.

Wanted: move them to a new `internal/hooks` package as `InstallHooks(root, target) (Status,
error)` and `InstallMergeAttribute(root)`, unit-tested on byte slices; `init.go` shrinks to about
120 lines. `hooks` imports `nodefile` for atomic writes and must not import `cli` or `agents`.
This replaces the pure-move split of `init.go` (`split-init-go`, not filed).

## Acceptance

- `internal/hooks` holds the editors, with unit tests for JSON member order and CRLF preservation.
- `go list -f '{{join .Imports "\n"}}' ./internal/hooks/` lists neither `internal/cli` nor
  `internal/agents`.
- The `init` end-to-end tests pass unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`, then `./internal/cli/ -run Init`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-09.
