---
title: One generic mv, rm and create module for both Tree kinds
status: open
type: task
priority: 3
deps: [issue-lifecycle-module, nodestore-post-write]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

The Tree-node verbs are written twice in `cli`: `mv.go` against `know_mv.go` (nearly line for
line), `rm.go` against `know_rm.go`, and the `update` and `create`/`know add` pairs. Each pair
differs only in the `nodestore.Kind`, the result builder, the Unread-warning wording ("deps" or
"superseded-by"), the post-write AGENTS.md refresh and whether `--kv` is on. The pairs have
already drifted in form: the Unread warning is inlined on one side and a helper on the other.
Strength: Strong for `mv` and `rm`, worth exploring for `update` and `create`.

Wanted: a generic descriptor `nodeVerbs[T]` holding the Kind constructor, the noun, the
write-result builder, an `afterWrite func(*Result)` and a `kv` switch, plus `newMvCommand[T]` and
`newRmCommand[T]`. `rm`'s `--drop-deps` and `DependentsError` become an optional extension. The
link-key name ("deps" or "superseded-by") moves into `nodestore.Codec` beside `Noun`. It builds on
`issue-lifecycle-module` and `nodestore-post-write`.

Risks: help text and flag sets are user-visible. `--kv` is registered only on commands whose
results carry KV records (closed issue `kv-flag-scope`), so the Note verbs stay without it.

## Acceptance

- `mv` and `know mv` are built by one generic constructor, and so are `rm` and `know rm`.
- `--help` output of `mv`, `know mv`, `rm` and `know rm` is unchanged, and `know mv` and
  `know rm` still have no `--kv`.
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`, then `./internal/cli/`, pass with the CLI tests
  unchanged.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-02.
