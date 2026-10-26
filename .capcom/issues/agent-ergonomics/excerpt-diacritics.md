---
title: Search excerpts miss words that differ only by diacritics
status: done
type: bug
priority: 2
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-26T09:46:10Z
closed: 2026-10-26T13:30:36Z
reason: Resolved by plan 2026-10-26 - 01 - p1-p2-ergonomics-plan (archived 6c800bf)
---

`index.Excerpt` (internal/index/excerpt.go:32) splits the query and body with `textTokens`
(internal/index/excerpt.go:66), which only lower-cases; the FTS5 `unicode61` tokenizer behind
the search also folds diacritics. So a search for `cafe` finds a body holding `café`, but
`--excerpt` prints no lines for that hit, as no excerpt token equals `cafe`.

## Acceptance

- `capcom search cafe --excerpt` on an issue whose body holds `café` prints that body line.
- The same holds for `capcom know search --excerpt`.
- A test in internal/index covers a query and body differing only by diacritics.
