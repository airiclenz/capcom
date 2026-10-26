---
title: Search returns nothing unless every word matches
status: done
type: feature
priority: 2
created: 2026-10-19T14:54:44Z
closed: 2026-10-26T09:47:07Z
reason: "Resolved by field-eval-ergonomics plan (archived 39bac45), item 2: search and know search fall back to any-word matches with match all/any in --json"
---

`capcom search` and `capcom know search` match only items that contain every word
(`matchExpression` in `internal/index/index.go`). An agent that searches with a natural phrase,
such as "how does the lock expire", gets nothing, and small models conclude there is nothing to
find. The results are already ranked by FTS5 rank.

Wanted: when the every-word query returns nothing, run an any-word query (FTS5 `OR`) and print
its ranked results under a plain heading saying they match some of the words, not all. The
`--json` envelope marks which query produced the results. Every-word matches keep today's
output exactly.

## Acceptance

- A query with one word missing from every item still returns the items holding the others,
  ranked, under the "some of the words" heading.
- A query whose words all match somewhere returns exactly what it returns today.
- FTS5 operator words in the query stay literal in both modes.
- Tests in `internal/index` and the CLI cover both modes for issues and Notes.
