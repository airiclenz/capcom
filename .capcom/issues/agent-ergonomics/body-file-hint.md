---
title: A missing --body-file hints capcom search
status: open
type: bug
priority: 2
plan: 2026-10-26 - 02 - p2-hint-closed-tab-plan
created: 2026-10-26T13:29:42Z
---

A `--body-file` (or `--body-append-file`) naming a file that does not exist fails with
`not_found` from `readTextInput` (internal/cli/node_edit.go:155), and so takes the default
`not_found` hint `capcom search <text>` (internal/cli/output/errors.go:36). Searching the
store does not help with a missing input file: the hint should point at the file, not the store.

Deferred from the run of plan `2026-10-26 - 01 - p1-p2-ergonomics-plan` (item 7's NOTES).

## Acceptance

- A missing `--body-file` on `create`, `know add` and `know update`, and a missing
  `--body-append-file`, ends with a hint that fits a missing input file, not `capcom search <text>`,
  in text, `--json` and `--kv` output.
- A test in internal/cli pins that hint.
