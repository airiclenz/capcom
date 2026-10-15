---
title: Filter the sidebar with /
status: open
type: feature
priority: 2
deps: [ui-issue-tree, ui-ready-tab, ui-knowledge-tab]
created: 2026-10-15T20:03:24Z
---

`/` opens a filter line over the current sidebar. Free text matches title and body as `search`
does; `status:<name>` and `label:<name>` narrow by field, and terms combine. In a tree a match
keeps its ancestors so the shape stays readable. Esc clears the filter; the status bar shows the
filter in effect.

## Acceptance

- The filter works on the Issues tree, the Ready list and the Knowledge tree.
- Text matches agree with `capcom search` and `capcom know search` for the same words.
- An unknown status or label shows a message, not an empty list without explanation.
- Tests cover text, status, label, combined terms, and ancestors kept in a tree.
