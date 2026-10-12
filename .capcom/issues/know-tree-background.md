---
title: know tree cannot hide background Notes such as rejected options
status: done
type: feature
priority: 2
deps: [outline-depth-hint]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:44Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 12 (archived in e61d7bf)
---

The rejected-option sections of decision records sit in `know tree` beside the decisions, and
smaller models read them as current guidance (knowledge-organization review, finding 3). A Note
labelled `background` is left out of `know tree`, subtree included, unless `--background` is
given, the way retired Notes are; a Note named as the start is still printed. The AGENTS block
skips background top-level Notes. `know show` and `know search` still find them. This uses a
label, so the store format is unchanged.

## Acceptance

- A background child is missing from `know tree` and present with `--background`.
- `know tree <background-note>` prints it; depth hints do not count hidden background Notes.
- `capcom agents` leaves a background top-level Note out of the block.
