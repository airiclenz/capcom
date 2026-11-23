---
title: Move the selection of an Issue's Notes out of internal/cli into internal/briefing
status: open
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
---

`capcom context` decides which Notes apply to an Issue (Notes whose refs name a path the body
mentions, then title-search hits) in `internal/cli/context.go` (`applyingNotes`,
`mentionsPath`, `endsPath`, `isPathByte`), and the every-word-then-any-word search fallback
`searchEveryWordFirst` sits in `internal/cli/search.go`. Move the selection to
`internal/briefing` and the fallback to `internal/index`, with direct unit tests. Item 2 of
the plan.

## Why

The selection is what capcom tells an agent about its work, which internal/briefing owns.

## Acceptance

- `internal/briefing` selects the applying Notes and `internal/index` holds the search fallback, each with direct tests
- `context`, `search` and `know search` print byte-identical output
