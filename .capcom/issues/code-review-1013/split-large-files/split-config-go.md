---
title: Split config/config.go into config, parse, format
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`internal/config/config.go` (498 lines) holds the config types, the format raise and a strict
YAML parser in one file. Split it:

- `config.go`: the types, `Default` and the lookups.
- `format.go`: `RaiseFormat` and `findFormatLine`.
- `parse.go`: the strict YAML parser.

This split comes before `config-owns-format`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/config` holds `format.go` and `parse.go` beside `config.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/config/` passes with the
  existing tests unchanged.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-05.
