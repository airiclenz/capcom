---
title: Split cli/know.go per subcommand
status: open
type: task
priority: 3
deps: [cli-flags-file]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

`internal/cli/know.go` (553 lines) holds the `know` root and five of its subcommands, while
`know update`, `know mv` and `know rm` already have files of their own. Split it per subcommand,
matching that naming:

- `know.go`: the `know` root command.
- `know_tree.go`: `know tree` and the outline.
- `know_show.go`: `know show` (`noteShowResult`).
- `know_search.go`: `know search`.
- `know_add.go`: `know add` and `refreshAgentsAfterWrite`.
- `know_verify.go`: `know verify`.

Update `internal/cli/doc.go`'s file map. This split comes after `cli-flags-file` and before
`cli-output-package`, `generic-node-verbs`, `nodestore-post-write` and `outline-walker`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/cli` holds `know_tree.go`, `know_show.go`, `know_search.go`, `know_add.go` and
  `know_verify.go` beside `know.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/` passes with the existing
  tests unchanged, the `doc.go` structural test included.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers and the `doc.go` map.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-08.
