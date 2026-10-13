---
title: Move close, reopen and status rules into the issue package
status: open
type: task
priority: 3
deps: [split-close-go, node-edit-helpers]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

The Issue status-category state machine lives only in cobra handlers: close needs a reason and a
time, reopen clears them and writes a Log entry, an update may not enter or leave the closed
category (`checkStatusChange`, `isClosedCategory`), and create must not start closed. "Category
is done or dropped" is written four times: `cli`'s `isClosedCategory`, `issue`'s `isClosed`,
`issue`'s validation and the index SQL. The interface a caller must learn is the whole of
`close.go`, and any future non-CLI caller could bypass the rules. Strength: Strong.

Wanted: add `config.Category.IsClosed()`. Add `issue.Close(i, status, reason, at)`,
`issue.Reopen(i, status, reason, author, at)` and `issue.SetStatus(i, to, cfg)`, which return a
new Issue or a typed error (`ErrAlreadyClosed`, `ErrClosedTransition`). The CLI keeps the flag
handling and the mapping to error Codes. Calling `issue.Close` on the fresh Issue inside
`Kind.Edit`'s change callback absorbs the close half of `dep-close-race`. ADR 0003 (rules read the
category, never the status name) is reinforced, not changed.

Risks: error wording and Codes must stay byte-identical; the CLI tests pin them.

## Acceptance

- `issue` unit tests cover `Close`, `Reopen` and `SetStatus`, including both typed errors,
  without cobra or git.
- The Go copies of the closed-category predicate are replaced by `config.Category.IsClosed`.
- The `close`, `reopen`, `update` and `create` CLI tests pass unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/config/`, then `./internal/issue/` and `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-01.
