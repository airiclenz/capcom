---
title: An issue body can only come from a file or stdin
status: open
type: feature
priority: 2
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-19T14:54:45Z
---

`capcom create` takes a body only through `--body-file` (a path or `-` for stdin), and
`update` appends through `--body-append` or `--body-append-file`. Heredocs and stdin are a
frequent failure point for small models in agent harnesses, so they file Issues without a body
and get the "no body" warning, or with a malformed one.

Wanted on `create`:
- `--body <text>`: the body, taken literally.
- `--acceptance <text>`, repeatable: each becomes a bullet under a `## Acceptance` heading that
  capcom appends to the body.
- `--why <text>`: a `## Why` section.

`--body` and `--body-file` are mutually exclusive. The "no body" warning names these flags.

## Acceptance

- `capcom create --id x --title T --body "What." --why "Because." --acceptance A --acceptance B`
  writes a body with the text, a `## Why` section and an `## Acceptance` list of A and B.
- `--body` with `--body-file` is refused before anything is written.
- The `create` examples show the inline form; CLI tests cover it.
