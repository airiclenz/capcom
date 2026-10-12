---
title: create accepts an issue without a body silently
status: done
type: feature
priority: 2
deps: [create-dep-flag]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 7 (archived in e61d7bf)
---

An issue filed with only a title can't be understood without reading something else, such as a
plan that may later be archived. `capcom create` without a body still succeeds but warns:
`<id> has no body: state what, why and a "## Acceptance" section with capcom update <id> --body-append-file <file>`.

## Acceptance

- A create without `--body-file`, or with a whitespace-only body, prints that exact warning in text
  and lists it in JSON `warnings`.
- A create with a body prints no such warning.
