---
title: Move the refs a removal breaks out of internal/cli into internal/knowledge
status: done
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
closed: 2026-11-23T08:56:56Z
reason: Resolved by plan 2026-11-23 - 01 - internal-cli-split-plan, item 8 (archived in b56a611)
---

After `rm` or `know rm`, `internal/cli/rm.go` (`brokenRefWarnings`) selects the Note refs
that name the removed file or its children directory. Move that selection to
`internal/knowledge`, beside `RefsNaming` and `Verify`, with direct unit tests; cli keeps
listing the Notes and wording the warnings. Item 8 of the plan.

## Why

Which refs a removal breaks is a Knowledge rule; it should be testable without a CLI run.

## Acceptance

- `internal/knowledge` selects the refs with a direct test; `internal/cli` no longer calls RefsNaming
- `rm` and `know rm` warnings stay byte-identical
