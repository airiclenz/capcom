---
title: One visible-subtree outline for issues and Notes
status: open
type: task
priority: 3
deps: [know-tree-orphans]
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
---

`list --tree`, `know tree` and `know show` each build an outline of their kind's nodes, grouping
children under parents, hiding what the command does not show (closed issues, background or
retired Notes) and choosing where the outline starts, in separate code in `internal/cli`
(`list.go`, `know.go`, `know_tree.go`). The grouping and hiding rules are the same shape per
kind and have already diverged (know tree dropped Notes below a Note it does not show).

Wanted: one generic outline type in `internal/cli/outline.go` with a per-command visibility
rule; the three commands group, hide and start their outlines through it. Pure refactor:
output unchanged.

## Acceptance

- `list --tree`, `know tree` and `know show` build their outlines through the one outline type;
  no per-command grouping or hiding code remains.
- Their output is unchanged; `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/`
  passes.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, Smaller locality fixes.
