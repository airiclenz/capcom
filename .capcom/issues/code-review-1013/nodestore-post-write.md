---
title: Format raise and AGENTS.md re-sync enforced by nodestore
status: open
type: task
priority: 3
deps: [config-owns-format, reference-rewrite-engine]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

Two store rules are re-implemented per command: "a write that adds a format-N key raises
`config.yaml` first" (`raiseStoreFormat` inside the `Kind.Edit` change callback of `related add`
and `know update`), and "a write to Knowledge's top level re-syncs AGENTS.md"
(`refreshAgentsAfterWrite`, called separately by `know add` at the top level, `know update`,
`know mv` and `know rm`). The copies already differ: `know update` validates before it raises the
format and `related add` does not. That is harmless today only because `related add`'s change
cannot invalidate an Issue the index accepted. Strength: Strong.

Wanted: `nodestore` enforces the order once, validate, then raise the format, then re-check, then
write: a hook in `Kind.Edit` and `Kind.Create` after validation, or a `Codec` method
`RequiredFormat(before, after T) int` reading `config.FormatRelated` and
`config.FormatSupersession` (from `config-owns-format`). The Notes Kind returns the AGENTS.md sync
status alongside `Written`. Keep the effects behind a function field or interface the CLI
supplies, so `nodestore` does not import `agents`; a production adapter plus a test adapter meets
the house rule for interfaces. ADR 0009's format rules are unchanged, only enforced in one place.
`reference-rewrite-engine` comes first (recommended order).

## Acceptance

- No `cli` command calls `raiseStoreFormat` or `refreshAgentsAfterWrite` itself; `nodestore`
  applies both effects.
- A `nodestore` test with a test adapter asserts the order validate, raise format, write, and that a
  change that fails validation raises no format.
- `go list -f '{{join .Imports "\n"}}' ./internal/nodestore/` does not list `internal/agents`.
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`, then `./internal/cli/`, pass, with the CLI output
  unchanged.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-03.
