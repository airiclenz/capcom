---
title: Managed block lists Notes the index leaves out
status: open
type: bug
priority: 2
deps: [kind-reader]
plan: 2026-10-14 - 02 - architecture-review-2-plan
created: 2026-10-14T16:53:33Z
---

`topNotes` (`internal/agents/block.go:150`) reads the Knowledge tree with its own parser and never
validates, so a top-level Note that the index leaves out (for example one with `labels: [""]`)
still appears in the AGENTS.md Managed block, while `know tree` and `know show` treat it as
unusable. The block then advertises a Note an agent cannot open.

Wanted (ratified 2026-10-14): the block reads Notes through the Kind reader and lists only Notes
it judges usable, so it hides what `know tree` hides. `capcom check` still reports the bad file.

## Acceptance

- A test with a top-level Note carrying `labels: [""]` shows it absent from both the Managed block
  and `know tree`, and reported by `capcom check`.
- The CHANGELOG `Unreleased` section has a line for the change.

Source: docs/reviews/architecture-review-2026-10-14.html, 3 · One node reader and descriptor per kind.
