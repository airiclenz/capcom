---
title: know show does not resolve a Note by its title
status: done
type: feature
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
closed: 2026-11-22T21:02:25Z
reason: "Resolved by plan 2026-11-22 - 01 - session-friction-followups-plan (daf400a), item 8: know show falls back to the title, the part before a colon, and the slugged argument; several matches fail naming each."
---

`capcom know show` takes an id, an alias or a unique id prefix. Agents that know a term or
a record number pass the title.

Seen: `capcom know show "<Glossary term>"` and `capcom know show "ADR <number>"`, six
distinct arguments. An argument that is an alias resolves; three were not and failed with
"id ... not found; did you mean ..." at the cost of a second call. One, of the form
"Release steps", was the start of the title of a Note whose id is its slug (`release-steps`),
which the suggestion did find.

Wanted: when the argument matches no id, alias or prefix, try it against the titles:
exact, case-insensitive, and the part of a title before a colon (glossary titles read
"Term: description"); also the argument turned into a slug ("Release steps" to
`release-steps`). Resolve only when exactly one Note matches. The same for `capcom show`
if it costs nothing.

## Acceptance

- `capcom know show "Release steps"` prints the Note `release-steps`.
- Two Notes with the same title give the not-found error naming both.
- `--help` says what an argument may be.
