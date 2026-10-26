---
title: Not every error names the command that fixes it
status: done
type: feature
priority: 2
deps: [suggest-ids-values]
plan: 2026-10-26 - 02 - p2-hint-closed-tab-plan
created: 2026-10-19T14:54:45Z
closed: 2026-10-26T14:38:05Z
reason: "resolved by plan 2026-10-26 - 02 - p2-hint-closed-tab-plan (archived 2035de6), item 1: the last hint gap (missing --body-file) fixed"
---

Some errors already name the command that resolves them (`status "done" is closed; use
reopen ...`, the "no body" warning), but not all do. An agent, small or large, recovers in one
step when the error carries the exact next command and loops when it does not.

Wanted: every user-facing error a write or read command can return ends with the command that
resolves it, with placeholders in angle brackets, for example
`hint: capcom reopen <id> --reason <text>`. Errors with no single fix (a corrupt file) name the
command that diagnoses them (`capcom check`). The `--json` error envelope carries the hint in
its own field.

Start from `internal/cli/errors.go` and `internal/cli/output`; list every error code and its
hint in one table so a test can walk it.

## Acceptance

- Every error code has a hint, held in one table.
- A test fails when a new error code is added without a hint.
- The hint appears as the last line in text output and as a field in `--json`.

## Log

- 2026-10-26 Airic Lenz: Partly landed via plan 2026-10-26 - 01 - p1-p2-ergonomics-plan (archived 6c800bf), items 5-7: default hint table per code, test for missing hints, hint line in text and field in --json. Left open: a missing --body-file still gets not_found's default hint 'capcom search <text>', which does not fit that error.
