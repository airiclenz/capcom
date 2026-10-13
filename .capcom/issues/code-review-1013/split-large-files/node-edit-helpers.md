---
title: Move shared edit helpers out of cli/update.go
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`internal/cli/update.go` (288 lines) hosts `editNode`, `readTextInput` and `writeResult`, which
`log`, `close`, `related`, `dep` and `know update` all use, so those commands reach into the
`update` command's file. Move the three helpers to a new `internal/cli/node_edit.go`;
`update.go` keeps the `update` command. Update `internal/cli/doc.go`'s file map. This prepares
`issue-lifecycle-module` and `generic-node-verbs`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/cli/node_edit.go` holds `editNode`, `readTextInput` and `writeResult`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/` passes with the existing
  tests unchanged, the `doc.go` structural test included.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers and the `doc.go` map.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-11.
