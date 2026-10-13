---
title: Split check/check.go into problem, issues, notes, agents
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
---

`internal/check/check.go` (565 lines) holds every rule `check` runs, for Issues, Notes and
AGENTS.md, in one file. Split it:

- `problem.go`: the `Problem` type and the Code constants, and the mapping of tree-load errors
  to Problems.
- `issues.go`: the Issue rules (parse, fields, plan lookup, deps, related, cycles).
- `notes.go`: the Note rules (format gate, supersession, retired subtrees, Ref checks).
- `agents.go`: the AGENTS.md check.
- `check.go` keeps `Run`.

This split comes before `config-owns-format` and `shared-diagnostics`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/check` holds `problem.go`, `issues.go`, `notes.go` and `agents.go` beside `check.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/check/` passes with the existing
  tests unchanged.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-04.
