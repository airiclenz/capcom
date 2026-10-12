---
title: Naming scheme, legacy docs, flagged ambiguities and config semantics are under-documented
status: done
type: task
priority: 2
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:44Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 15 (archived in e61d7bf)
---

Four tidy-ups from the knowledge-organization review:
- (2) `docs/format.md` explains the naming scheme with a worked example from this repo, and says a
  child id repeating its parent's id is a habit, not a rule.
- (4) Note `docs-layout` says `CONTEXT.md` and `docs/adr/README.md` only point to Notes, and
  `docs/design/research-notes.md` is the pre-v1 research record.
- (5) Note `glossary` says what a flagged ambiguity is.
- (6) A new Note `store-config` under `store-layout` summarises what `config.yaml` controls and
  points to the reference.

## Acceptance

- The four texts are present; `capcom know show store-config` prints the new Note.
- `capcom check` and `capcom fmt --check` report no problems; `capcom know verify` reports no new
  broken ref.
